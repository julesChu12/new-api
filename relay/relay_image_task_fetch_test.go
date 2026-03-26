package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRelayImageTaskFetchTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Task{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestRelayImageTaskFetchByID_ReturnsImageTaskPayload(t *testing.T) {
	db := setupRelayImageTaskFetchTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_image_fetch_1",
		UserId:    7,
		Platform:  constant.TaskPlatform("58"),
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		ChannelId: 58,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://example.com/final.png",
		},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "task_id", Value: "task_image_fetch_1"}}
	ctx.Set("id", 7)

	taskErr := RelayImageTaskFetchByID(ctx)
	require.Nil(t, taskErr)
	assert.Equal(t, http.StatusOK, recorder.Code)

	var payload dto.TaskResponse[dto.ImageTaskResponse]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "success", payload.Code)
	assert.Equal(t, "task_image_fetch_1", payload.Data.TaskID)
	assert.Equal(t, "succeeded", payload.Data.Status)
	assert.Equal(t, "https://example.com/final.png", payload.Data.Url)
	assert.Nil(t, payload.Data.Error)
}

func TestRelayImageTaskFetchByID_ReturnsFailurePayloadWithoutFakeURL(t *testing.T) {
	db := setupRelayImageTaskFetchTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:     "task_image_fetch_failure",
		UserId:     8,
		Platform:   constant.TaskPlatform("58"),
		Status:     model.TaskStatusFailure,
		Progress:   "100%",
		ChannelId:  58,
		FailReason: "upstream failed",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "task_id", Value: "task_image_fetch_failure"}}
	ctx.Set("id", 8)

	taskErr := RelayImageTaskFetchByID(ctx)
	require.Nil(t, taskErr)
	assert.Equal(t, http.StatusOK, recorder.Code)

	var payload dto.TaskResponse[dto.ImageTaskResponse]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "failed", payload.Data.Status)
	assert.Empty(t, payload.Data.Url)
	require.NotNil(t, payload.Data.Error)
	assert.Equal(t, "upstream failed", payload.Data.Error.Message)
}
