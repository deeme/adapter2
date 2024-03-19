package coze

import (
	"errors"
	"github.com/bincooo/chatgpt-adapter/v2/internal/agent"
	"github.com/bincooo/chatgpt-adapter/v2/internal/middle"
	"github.com/bincooo/chatgpt-adapter/v2/pkg/gpt"
	"github.com/bincooo/coze-api"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

func completeToolCalls(ctx *gin.Context, cookie, proxies string, req gpt.ChatCompletionRequest) (bool, error) {
	logrus.Infof("completeTools ...")
	prompt, err := middle.BuildToolCallsTemplate(
		req.Tools,
		req.Messages,
		agent.CQConditions, 5)
	if err != nil {
		return false, err
	}

	pMessages := []coze.Message{
		{
			Role:    "system",
			Content: prompt,
		},
	}

	ck := ""
	msToken := ""
	if !strings.Contains(cookie, "[msToken=") {
		return false, errors.New("please provide the '[msToken=xxx]' cookie parameter")
	} else {
		co := strings.Split(cookie, "[msToken=")
		msToken = strings.TrimSuffix(co[1], "]")
		ck = co[0]
	}

	options := newOptions(proxies, pMessages)
	chat := coze.New(ck, msToken, options)
	chatResponse, err := chat.Reply(ctx.Request.Context(), pMessages)
	if err != nil {
		return false, err
	}

	content, err := waitMessage(chatResponse)
	if err != nil {
		return false, err
	}
	logrus.Infof("completeTools response: \n%s", content)

	var fun *gpt.Function
	for _, t := range req.Tools {
		if strings.Contains(content, t.Fun.Id) {
			fun = &t.Fun
		}
	}

	// 不是工具调用
	if fun == nil {
		return true, nil
	}

	// 收集参数
	return parseToToolCall(ctx, cookie, proxies, fun, req.Messages, req.Stream)
}

func parseToToolCall(ctx *gin.Context, cookie, proxies string, fun *gpt.Function, messages []map[string]string, sse bool) (bool, error) {
	logrus.Infof("parseToToolCall ...")
	prompt, err := middle.BuildToolCallsTemplate(
		[]struct {
			Fun gpt.Function `json:"function"`
			T   string       `json:"type"`
		}{{Fun: *fun, T: "function"}},
		messages,
		agent.ExtractJson, 5)
	if err != nil {
		return false, err
	}

	pMessages := []coze.Message{
		{
			Role:    "system",
			Content: prompt,
		},
	}

	ck := ""
	msToken := ""
	if !strings.Contains(cookie, "[msToken=") {
		return false, errors.New("please provide the '[msToken=xxx]' cookie parameter")
	} else {
		co := strings.Split(cookie, "[msToken=")
		msToken = strings.TrimSuffix(co[1], "]")
		ck = co[0]
	}

	options := newOptions(proxies, pMessages)
	chat := coze.New(ck, msToken, options)
	chatResponse, err := chat.Reply(ctx.Request.Context(), pMessages)
	if err != nil {
		return false, err
	}

	content, err := waitMessage(chatResponse)
	if err != nil {
		return false, err
	}
	logrus.Infof("parseToToolCall response: \n%s", content)

	created := time.Now().Unix()
	left := strings.Index(content, "{")
	right := strings.LastIndex(content, "}")
	argv := ""
	if left >= 0 && right > left {
		argv = content[left : right+1]
	}

	if sse {
		middle.ResponseWithSSEToolCalls(ctx, MODEL, fun.Name, argv, created)
		return false, nil
	} else {
		middle.ResponseWithToolCalls(ctx, MODEL, fun.Name, argv)
		return false, nil
	}
}
