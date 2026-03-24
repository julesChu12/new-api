package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTencentVODCallbackControllerTestDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}, &model.User{}, &model.Token{}, &model.Log{}, &model.UserSubscription{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestTencentVODCallback_RejectsInvalidToken(t *testing.T) {
	db := setupTencentVODCallbackControllerTestDB(t)
	callbackSecret := `{"tencent_vod_callback_secret":"expected-token","tencent_vod_callback_enabled":true}`
	require.NoError(t, db.Create(&model.Channel{
		Id:            58,
		Type:          constant.ChannelTypeTencentVOD,
		Name:          "tencent-vod-test",
		Key:           "sid|skey",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: callbackSecret,
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:         "task_public_callback_1",
		Platform:       constant.TaskPlatform("58"),
		UpstreamTaskID: "upstream_1",
		ChannelId:      58,
		Status:         model.TaskStatusSubmitted,
		Progress:       "0%",
		Group:          "default",
	}).Error)

	gin.SetMode(gin.TestMode)
	payload := []byte(`{"EventType":"ProcedureStateChanged","ProcedureStateChangeEvent":{"TaskId":"upstream_1","Status":"FINISH"}}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/tencent_vod/callback?token=wrong-token", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")

	TencentVODCallback(ctx)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestTencentVODCallback_SuccessfullyCompletesTask(t *testing.T) {
	db := setupTencentVODCallbackControllerTestDB(t)
	callbackSecret := `{"tencent_vod_callback_secret":"expected-token","tencent_vod_callback_enabled":true}`
	require.NoError(t, db.Create(&model.Channel{
		Id:            59,
		Type:          constant.ChannelTypeTencentVOD,
		Name:          "tencent-vod-success",
		Key:           "sid|skey",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: callbackSecret,
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:         "task_public_callback_success",
		Platform:       constant.TaskPlatform("58"),
		UpstreamTaskID: "upstream_success_callback",
		ChannelId:      59,
		Status:         model.TaskStatusInProgress,
		Progress:       "50%",
		Group:          "default",
	}).Error)

	payload := []byte(`{"EventType":"ProcedureStateChanged","ProcedureStateChangeEvent":{"TaskId":"upstream_success_callback","Status":"FINISH","FileUrl":"https://example.com/success.png","Progress":100}}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/tencent_vod/callback?token=expected-token", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")

	TencentVODCallback(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_public_callback_success").First(&task).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, task.Status)
	assert.Equal(t, "100%", task.Progress)
	assert.Equal(t, "https://example.com/success.png", task.PrivateData.ResultURL)
	assert.NotZero(t, task.FinishTime)
}

func TestTencentVODCallback_DuplicateFailureDoesNotRefundTwice(t *testing.T) {
	db := setupTencentVODCallbackControllerTestDB(t)
	callbackSecret := `{"tencent_vod_callback_secret":"expected-token","tencent_vod_callback_enabled":true}`
	require.NoError(t, db.Create(&model.User{Id: 9, Username: "callback-user", Quota: 10000, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 9, UserId: 9, Key: "callback-token", Name: "callback-token", Status: common.TokenStatusEnabled, RemainQuota: 6000}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:            60,
		Type:          constant.ChannelTypeTencentVOD,
		Name:          "tencent-vod-failure",
		Key:           "sid|skey",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: callbackSecret,
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:         "task_public_callback_failure",
		Platform:       constant.TaskPlatform("58"),
		UpstreamTaskID: "upstream_failure_callback",
		UserId:         9,
		ChannelId:      60,
		Quota:          3000,
		Status:         model.TaskStatusInProgress,
		Progress:       "50%",
		Group:          "default",
		PrivateData: model.TaskPrivateData{
			BillingSource: service.BillingSourceWallet,
			TokenId:       9,
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "test-model",
				ModelPrice:      0.02,
				GroupRatio:      1,
			},
		},
		Properties: model.Properties{OriginModelName: "test-model"},
	}).Error)

	payload := []byte(`{"EventType":"ProcedureStateChanged","ProcedureStateChangeEvent":{"TaskId":"upstream_failure_callback","Status":"ABORTED","ErrMsg":"failed upstream","Progress":100}}`)

	call := func() {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/tencent_vod/callback?token=expected-token", bytes.NewReader(payload))
		ctx.Request.Header.Set("Content-Type", "application/json")
		TencentVODCallback(ctx)
		assert.Equal(t, http.StatusOK, recorder.Code)
	}

	call()
	call()

	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_public_callback_failure").First(&task).Error)
	assert.EqualValues(t, model.TaskStatusFailure, task.Status)
	assert.Equal(t, 13000, getControllerUserQuota(t, db, 9))
	assert.Equal(t, int64(1), getControllerLogCount(t, db))
}

func getControllerUserQuota(t *testing.T, db *gorm.DB, id int) int {
	t.Helper()
	var user model.User
	require.NoError(t, db.Select("quota").Where("id = ?", id).First(&user).Error)
	return user.Quota
}

func getControllerLogCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&model.Log{}).Count(&count).Error)
	return count
}
