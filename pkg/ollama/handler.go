package ollama

import (
	"bytes"
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
	cli   *api.Client
	Debug bool
}

func (c *ChatHandler) SetDebug(debug bool) {
	c.Debug = debug
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

func (c *ChatHandler) Streamable(isStreamFn func(ctx *gin.Context) bool, noStreamFn func(ctx *gin.Context, msgs [][]byte) []byte, fn func(c *ChatHandler, ctx *gin.Context, isStream bool, msg chan []byte, done chan struct{}) error) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		body := GetBodyFromContext(ctx)
		log.Println(ctx.Request.Method, ctx.Request.URL.RawPath, "body", string(body), "contextLength", len(string(body)))
		stream := isStreamFn(ctx)
		if stream {
			ctx.Header("Content-Type", "application/x-ndjson")
		} else {
			ctx.Header("Content-Type", "application/json")
		}
		msgChan := make(chan []byte)
		doneChan := make(chan struct{})
		res := make([][]byte, 0)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case data := <-msgChan:
					res = append(res, data)
					if stream {
						if c.Debug {
							log.Printf("resp stream: %s", string(data))
						}
						ctx.Writer.Write(data)
					}
				case <-doneChan:
					return
				}
				if stream {
					ctx.Writer.Write([]byte("\n"))
				}
			}
		}()
		if err := fn(c, ctx, stream, msgChan, doneChan); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		wg.Wait()
		close(msgChan)
		for data := range msgChan {
			res = append(res, data)
			if stream {
				if c.Debug {
					log.Printf("resp stream: %s", string(data))
				}
				ctx.Writer.Write(data)
			}
		}
		if !stream {
			if data := noStreamFn(ctx, res); data != nil {
				log.Printf("resp no stream: %s, contentLength: %v", string(data), len(string(data)))
				ctx.Writer.Write(data)
			} else {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "no response"})
			}
		} else {
			result := Response{}
			for _, r := range res {
				m := Response{}
				_ = json.Unmarshal(r, &m)
				result.Message.Content += m.Message.Content
				if toolCalls := m.Message.ToolCalls; toolCalls != nil {
					result.Message.ToolCalls = append(result.Message.ToolCalls, toolCalls...)
				}
				result.Response += m.Response
			}
			log.Printf("resp: %+v", result)
		}
		return
	}
}

type Response struct {
	Message struct {
		Content   string         `json:"content"`
		ToolCalls []api.ToolCall `json:"tool_calls,omitempty"`
	}
	Response string `json:"response"`
}

func GetBodyFromContext(ctx *gin.Context) []byte {
	body, _ := io.ReadAll(ctx.Request.Body)
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer([]byte(body)))
	return body
}

func (c *ChatHandler) EmbeddingsFunc(ctx *gin.Context) {
	req := &api.EmbeddingRequest{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		return
	}
	log.Println("embedding", req.Prompt)
	resp, err := c.cli.Embeddings(ctx, req)
	if err != nil {
		return
	}
	data, _ := json.Marshal(resp)
	ctx.Writer.Write(data)
}

func (c *ChatHandler) GenerateFunc() func(ctx *gin.Context) {
	return c.Streamable(func(ctx *gin.Context) bool {
		body := GetBodyFromContext(ctx)
		m := make(map[string]interface{})
		_ = json.Unmarshal(body, &m)
		if stream, ok := m["stream"]; ok {
			if s, ok := stream.(bool); ok {
				return s
			}
		}
		return true
	}, func(ctx *gin.Context, msgs [][]byte) []byte {
		var message string
		res := make([]api.GenerateResponse, 0)
		for _, msg := range msgs {
			r := api.GenerateResponse{}
			_ = json.Unmarshal(msg, &r)
			res = append(res, r)
		}
		for _, r := range res {
			message += r.Response
		}
		var data []byte
		if len(res) > 0 {
			result := res[len(res)-1]
			result.Response = message
			data, _ = json.Marshal(result)
		}
		return data
	}, func(c *ChatHandler, ctx *gin.Context, isStream bool, msg chan []byte, done chan struct{}) error {
		req := &api.GenerateRequest{}
		if err := ctx.ShouldBindJSON(req); err != nil {
			return err
		}
		if err := c.cli.Generate(ctx, req, func(resp api.GenerateResponse) error {
			data, _ := json.Marshal(resp)
			msg <- data
			if resp.Done {
				close(done)
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	})
}
func (c *ChatHandler) ChatFunc() func(ctx *gin.Context) {
	return c.Streamable(func(ctx *gin.Context) bool {
		body := GetBodyFromContext(ctx)
		m := make(map[string]interface{})
		_ = json.Unmarshal(body, &m)
		if stream, ok := m["stream"]; ok {
			if s, ok := stream.(bool); ok {
				return s
			}
		}
		return true
	}, func(ctx *gin.Context, msgs [][]byte) []byte {
		var message string
		res := make([]api.ChatResponse, 0)
		for _, msg := range msgs {
			r := api.ChatResponse{}
			_ = json.Unmarshal(msg, &r)
			res = append(res, r)
		}
		for _, r := range res {
			message += r.Message.Content
		}
		var data []byte
		if len(res) > 0 {
			result := res[len(res)-1]
			result.Message.Content = message
			data, _ = json.Marshal(result)
		}
		return data
	}, func(c *ChatHandler, ctx *gin.Context, isStream bool, msg chan []byte, done chan struct{}) error {
		request := &api.ChatRequest{}
		data, err := io.ReadAll(ctx.Request.Body)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &request); err != nil {
			return err
		}
		if err := c.Chat(func(resp api.ChatResponse) error {
			data, _ := json.Marshal(resp)
			msg <- data
			if resp.Done {
				close(done)
			}
			return nil
		}, request); err != nil {
			return err
		}
		return nil
	})
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
