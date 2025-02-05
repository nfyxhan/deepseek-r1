package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/igm/sockjs-go/sockjs"
	"github.com/nfyxhan/deepseek-r1/pkg/ollama/api"
)

type ChatHandler struct {
	cli *api.Client
}

func NewChatHandler(uri string) (*ChatHandler, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	client := api.NewClient(u, http.DefaultClient)
	return &ChatHandler{
		cli: client,
	}, nil
}

func (c *ChatHandler) ChatFunc(ctx *gin.Context) {
	request := &api.ChatRequest{}
	data, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("req: %s", string(data))
	if err := json.Unmarshal(data, &request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	stream := true
	if s := request.Stream; s != nil && !*s {
		stream = *s
	}
	ctx.Header("Content-Type", "application/x-ndjson")
	msgChan := make(chan api.ChatResponse)
	res := make([]api.ChatResponse, 0)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case r := <-msgChan:
				if !stream {
					res = append(res, r)
				} else {
					data, _ = json.Marshal(r)
					log.Printf("resp: %s", string(data))
					ctx.Writer.Write(data)
				}
				if r.Done {
					return
				}
			}
			if stream {
				ctx.Writer.Write([]byte("\n"))
			}
		}
	}()
	if err := c.Chat(func(resp api.ChatResponse) error {
		// log.Printf("resp: %+v", resp)
		msgChan <- resp
		return nil
	}, request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wg.Wait()
	if !stream {
		var message string
		for _, r := range res {
			message += r.Message.Content
		}
		if len(res) > 0 {
			result := res[len(res)-1]
			result.Message.Content = message
			data, _ = json.Marshal(result)
			log.Printf("resp: %s", string(data))
			ctx.Writer.Write(data)
		} else {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "no response"})
		}
	}
	return
}

func (c *ChatHandler) ChatWebsocket(path string) func(*gin.Context) {
	return func(ctx *gin.Context) {
		sockHandler := func(session sockjs.Session) {
			defer func() {
				err := session.Close(0, "exit close 0")
				if err != nil {
					log.Printf("close session %s err: %s", session.ID(), err)
				}
			}()
			model := ctx.Param("model")
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
				if err := c.Chat(func(resp api.ChatResponse) error {
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
				}, &api.ChatRequest{
					Model:    model,
					Messages: messages,
				}); err != nil {
					_ = session.Send(fmt.Sprintf("session close for error: %s\n", err))
					log.Printf("session %s close for err: %s", session.ID(), err)
					break
				}
				if err := session.Send("\n"); err != nil {
					return
				}
				contents = append(contents, reply)
			}
			log.Printf("session %s closed", session.ID())
		}
		// handler of ${path}/info and ${path}/:a/:b/websocket
		sockjs.NewHandler(path, sockjs.DefaultOptions, sockHandler).ServeHTTP(ctx.Writer, ctx.Request)
	}
}

func (c *ChatHandler) Chat(respFunc func(resp api.ChatResponse) error, req *api.ChatRequest) error {
	ctx := context.Background()
	if err := c.cli.Chat(ctx, req, respFunc); err != nil {
		return err
	}
	return nil
}
