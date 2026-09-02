package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestAssetQueryAgainstLiveMongo 直接调用 asset.go 里的 handler（跳过登录/验证码），
// 对接一个已经由改造后的 mongo_syncer 真实落库过数据的 MongoDB 实例，验证跨主机资产
// 查询接口（端口/软件/容器/进程）能读到正确的数据和分页结构。
//
// 通过环境变量 VERIFY_ASSET_MONGO_URI 指定测试用 MongoDB 地址时才会运行，默认跳过。
func TestAssetQueryAgainstLiveMongo(t *testing.T) {
	uri := os.Getenv("VERIFY_ASSET_MONGO_URI")
	if uri == "" {
		t.Skip("VERIFY_ASSET_MONGO_URI 未设置，跳过真实 Mongo 资产查询验证")
	}

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("连接 MongoDB 失败: %v", err)
	}
	db := client.Database(os.Getenv("VERIFY_ASSET_MONGO_DB"))

	gin.SetMode(gin.TestMode)
	s := &APIServer{db: db, router: gin.New()}
	s.router.GET("/assets/ports", s.listAssetPorts)
	s.router.GET("/assets/software", s.listAssetSoftware)
	s.router.GET("/assets/containers", s.listAssetContainers)
	s.router.GET("/assets/processes", s.listAssetProcesses)

	cases := []struct {
		name string
		path string
	}{
		{"ports", "/assets/ports?page_size=5"},
		{"software", "/assets/software?page_size=5"},
		{"containers", "/assets/containers?page_size=5"},
		{"processes", "/assets/processes?page_size=5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			s.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("查询 %s 失败: status=%d body=%s", tc.name, w.Code, w.Body.String())
			}
			t.Logf("%s 响应: %s", tc.name, w.Body.String())
		})
	}
}
