package ollama

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nfyxhan/deepseek-r1/assets"
	"github.com/nfyxhan/deepseek-r1/pkg/ollama/api"

	"github.com/gin-gonic/gin"
	"github.com/igm/sockjs-go/sockjs"
)

func ChatWebsocket(path string, uri string, model string) func(*gin.Context) {
	return func(ctx *gin.Context) {
		sockHandler := func(session sockjs.Session) {
			defer func() {
				err := session.Close(0, "exit close 0")
				if err != nil {
					log.Printf("close session %s err: %s", session.ID(), err)
				}
			}()
			reader, writer := io.Pipe()
			defer writer.Close()
			go func() {
				for {
					data := make([]byte, 1024)
					n, err := reader.Read(data)
					if err != nil {
						fmt.Println("err=", err)
						break
					}
					_ = session.Send(string(data[:n]))
				}
			}()
			contents := make([]string, 0)
			for {
				content, err := session.Recv()
				if err != nil {
					time.Sleep(time.Second)
					continue
				}
				log.Printf("%s", content)
				contents = append(contents, content)
				var reply string
				var think = true
				if err := Chat(uri, model, func(resp api.ChatResponse) error {
					fmt.Printf("%s", resp.Message.Content)
					if strings.Contains(resp.Message.Content, "</think>") {
						think = false
					}
					if !think {
						reply += resp.Message.Content
					}
					if err := session.Send(resp.Message.Content); err != nil {
						return err
					}
					return nil
				}, contents...); err != nil {
					_ = session.Send(fmt.Sprintf("session close for error: %s\n", err))
					log.Printf("session %s close for err: %s", session.ID(), err)
					break
				}
				fmt.Println()
				contents = append(contents, reply)
			}
			log.Printf("session %s closed", session.ID())
		}
		// handler of ${path}/info and ${path}/:a/:b/websocket
		sockjs.NewHandler(path, sockjs.DefaultOptions, sockHandler).ServeHTTP(ctx.Writer, ctx.Request)
	}
}

func ChatServer(uri string, model string, port string, du time.Duration) error {
	quit := make(chan os.Signal, 2)
	signal.Notify(
		quit,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	router := gin.New()
	router.POST("/chat", func(c *gin.Context) {
		body := struct {
			Contents []string `json:"contents"`
		}{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		Chat(uri, model, func(resp api.ChatResponse) error {
			c.JSON(http.StatusOK, resp)
			return nil
		}, body.Contents...)
	})
	prefixPath := "/chatwebsocket"
	handlers := []gin.HandlerFunc{
		ChatWebsocket(prefixPath, uri, model),
	}

	fs := http.FileServer(http.FS(assets.StaticFiles))
	router.GET("/static/*filepath", func(ctx *gin.Context) {
		w := ctx.Writer
		r := ctx.Request
		fs.ServeHTTP(w, r.WithContext(r.Context()))
	})
	router.GET(fmt.Sprintf("/%s/%s", prefixPath, ":a"), handlers...)
	router.GET(fmt.Sprintf("/%s/%s", prefixPath, ":a/:b/:c"), handlers...)
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
	var err error
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
func Chat(uri string, model string, respFunc func(resp api.ChatResponse) error, contents ...string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}
	client := api.NewClient(u, http.DefaultClient)

	messages := []api.Message{}
	for i, content := range contents {
		role := "assistant"
		if i%2 == 0 {
			role = "user"
		}
		messages = append(messages, api.Message{
			Role:    role,
			Content: content,
		})
	}
	ctx := context.Background()
	req := &api.ChatRequest{
		Model:    model,
		Messages: messages,
	}

	if err := client.Chat(ctx, req, respFunc); err != nil {
		return err
	}
	return nil
}
