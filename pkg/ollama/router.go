package ollama

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nfyxhan/deepseek-r1/assets"
	"github.com/nfyxhan/deepseek-r1/pkg/ollama/api"

	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/pprof"
)

func ChatServer(uri string, port string, qps float64, du time.Duration) error {
	quit := make(chan os.Signal, 2)
	signal.Notify(
		quit,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	router := gin.New()
	middlewares := []gin.HandlerFunc{gin.Logger()}
	router.NoMethod(middlewares...)
	// No router handler
	router.NoRoute(middlewares...)

	pprof.Register(router)

	h, err := NewChatHandler(uri)
	if err != nil {
		return err
	}
	router.Use(RateLimit(LimitByClientIP(""), qps, time.Minute))
	router.GET("/", func(ctx *gin.Context) {
		ctx.Writer.Write([]byte("Ollama is running"))
	})
	router.GET("/qps", func(ctx *gin.Context) {
		id := ctx.Query("id")
		qps := GetQps(id)
		ctx.Writer.Write([]byte(fmt.Sprintf("%v", qps)))
	})
	router.POST("/qps", func(ctx *gin.Context) {
		id := ctx.Query("id")
		qps, _ := strconv.ParseFloat(ctx.Query("qps"), 64)
		if qps < 0 {
			return
		}
		SetQps(id, qps)
		ctx.Writer.Write([]byte(fmt.Sprintf("%v", qps)))
	})

	serverApi := router.Group("/api", middlewares...)
	serverApi.POST("/chat", h.ChatFunc())
	serverApi.GET("/tags", func(ctx *gin.Context) {
		res, err := h.cli.List(ctx)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		result := make([]map[string]interface{}, 0)
		for _, r := range res.Models {
			result = append(result, map[string]interface{}{
				"model": r.Model,
			})
		}
		ctx.JSON(http.StatusOK, map[string]interface{}{"models": res.Models})
	})
	serverApi.POST("/generate", h.GenerateFunc())
	serverApi.POST("/embeddings", h.EmbeddingsFunc)
	serverApi.POST("/show", func(ctx *gin.Context) {
		req := &api.ShowRequest{}
		if err := ctx.ShouldBindJSON(req); err != nil {
			log.Println(err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := h.cli.Show(ctx, req)
		if err != nil {
			log.Println(err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, resp)
	})
	prefixPath := "/chatwebsocket/[a-z0-9\\.\\-]+[:][a-z0-9\\.]+"
	handlers := []gin.HandlerFunc{
		h.ChatWebsocket(prefixPath),
	}
	prefixPath = "/chatwebsocket/:model"
	router.GET(fmt.Sprintf("/%s/%s", prefixPath, ":a"), handlers...)
	router.GET(fmt.Sprintf("/%s/%s", prefixPath, ":a/:b/:c"), handlers...)

	fs := http.FileServer(http.FS(assets.StaticFiles))
	router.GET("/static/*filepath", func(ctx *gin.Context) {
		w := ctx.Writer
		r := ctx.Request
		fs.ServeHTTP(w, r.WithContext(r.Context()))
	})
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println(err)
			quit <- syscall.SIGTERM
		}
	}()
	s := <-quit
	log.Printf("server will stop in %s for syscall %s", du, s)
	ctx, cancel := context.WithTimeout(context.Background(), du)
	defer cancel()
	errChan := make(chan error)
	go func() {
		errChan <- srv.Shutdown(ctx)
	}()
	select {
	case <-ctx.Done():
		err = errors.New("shutdown timeout")
	case err = <-errChan:
	}
	if err != nil {
		return err
	}
	log.Println("Server Shutdown Successfully")
	return nil
}
