package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"infinite-canvas-server/model"
)

type ModelTestInput struct {
	Model          string `json:"model"`
	ChannelID      uint   `json:"channel_id"`
	ChannelModelID uint   `json:"channel_model_id"`
	Generation     string `json:"generation"`
	Route          string `json:"route"`
	Prompt         string `json:"prompt"`
	Operation      string `json:"operation"`
	Size           string `json:"size"`
	AspectRatio    string `json:"aspect_ratio"`
	Seconds        int    `json:"seconds"`
	HasReferences  bool   `json:"has_references"`
	ReferenceCount int    `json:"reference_count"`
}

type ModelTestResult struct {
	Success        bool   `json:"success"`
	Model          string `json:"model"`
	Generation     string `json:"generation"`
	Route          string `json:"route"`
	Method         string `json:"method"`
	Path           string `json:"path"`
	StatusCode     int    `json:"status_code"`
	ResponseTimeMs int    `json:"response_time_ms"`
	ErrorMessage   string `json:"error_message"`
	ResponseBody   string `json:"response_body"`
}

type modelTestRequest struct {
	Generation  string
	Route       string
	Method      string
	Path        string
	ContentType string
	Body        []byte
}

const testReferenceImageURL = "https://dummyimage.com/1024x1024/ffffff/000000.png"

var testReferencePNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
	0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f,
	0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc, 0xcc, 0x59, 0xe7, 0x00, 0x00, 0x00,
	0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func (s *GenerateService) TestModel(tenantID, userID uint, input ModelTestInput) (*ModelTestResult, error) {
	modelName := strings.TrimSpace(input.Model)
	if modelName == "" {
		return nil, errors.New("请指定模型")
	}
	selection := ChannelSelection{ChannelID: input.ChannelID, ChannelModelID: input.ChannelModelID}
	generation := strings.TrimSpace(input.Generation)
	if generation == "" {
		generation = resolveGenerationByChannelModel(selection, modelName, s.modelRepo)
	}
	route, err := s.resolveChannelRoute(selection, generation, modelName)
	if err != nil {
		return nil, err
	}
	input.Generation = generation
	input.Route = routeForModelTest(input.Route, generation, route.ChannelModel)
	cfg := configForChannelModel(route.ChannelModel)

	testReq, err := buildModelTestRequest(cfg, input)
	if err != nil {
		return nil, err
	}
	if _, _, err := s.getRequiredPricing(tenantID, testReq.Generation, modelName, testReq.ContentType, testReq.Body); err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, testReq.Generation, modelName, testReq.Method, testReq.Path, 0, nil, err.Error(), route)
		return nil, err
	}

	req, err := http.NewRequest(testReq.Method, buildUpstreamURL(route.Channel.BaseUrl, testReq.Path), bytes.NewReader(testReq.Body))
	if err != nil {
		return nil, err
	}
	if testReq.ContentType != "" {
		req.Header.Set("Content-Type", testReq.ContentType)
	}
	req.Header.Set("Authorization", "Bearer "+route.ApiKey)

	startTime := time.Now()
	resp, err := s.httpClient.Do(req)
	responseTimeMs := int(time.Since(startTime).Milliseconds())
	if err != nil {
		message := fmt.Sprintf("上游 API 请求失败: %v", err)
		s.recordModelFailureWithRoute(tenantID, userID, testReq.Generation, modelName, testReq.Method, testReq.Path, 0, nil, message, route)
		return &ModelTestResult{
			Success:        false,
			Model:          modelName,
			Generation:     testReq.Generation,
			Route:          testReq.Route,
			Method:         testReq.Method,
			Path:           testReq.Path,
			StatusCode:     0,
			ResponseTimeMs: responseTimeMs,
			ErrorMessage:   message,
		}, nil
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, testReq.Generation, modelName, testReq.Method, testReq.Path, resp.StatusCode, nil, err.Error(), route)
		return nil, err
	}
	if resp.StatusCode < 400 {
		if converted, ok := transformImageResponseToChatFormat(testReq.Path, respBytes); ok {
			respBytes = converted
		}
	}

	result := &ModelTestResult{
		Success:        resp.StatusCode < 400,
		Model:          modelName,
		Generation:     testReq.Generation,
		Route:          testReq.Route,
		Method:         testReq.Method,
		Path:           testReq.Path,
		StatusCode:     resp.StatusCode,
		ResponseTimeMs: responseTimeMs,
		ResponseBody:   responseSnippet(resp.Header.Get("Content-Type"), respBytes),
	}
	if result.Success {
		s.recordModelSuccessWithRoute(tenantID, userID, testReq.Generation, modelName, testReq.Method, testReq.Path, resp.StatusCode, responseTimeMs, route)
	} else {
		result.ErrorMessage = buildModelCallErrorSummary(resp.StatusCode, respBytes, "")
		s.recordModelFailureWithRoute(tenantID, userID, testReq.Generation, modelName, testReq.Method, testReq.Path, resp.StatusCode, respBytes, "", route)
	}
	return result, nil
}

func routeForModelTest(override, generation string, item *model.ChannelModel) string {
	if strings.TrimSpace(override) != "" && strings.TrimSpace(override) != "auto" {
		return strings.TrimSpace(override)
	}
	if item == nil {
		return ""
	}
	switch generation {
	case "image":
		if strings.TrimSpace(item.ImageGenerateRoute) != "" {
			return strings.TrimSpace(item.ImageGenerateRoute)
		}
	case "video":
		if strings.TrimSpace(item.VideoRoute) != "" {
			return strings.TrimSpace(item.VideoRoute)
		}
	}
	return ""
}

func configForChannelModel(item *model.ChannelModel) *model.TenantApiConfig {
	cfg := &model.TenantApiConfig{}
	if item == nil {
		return cfg
	}
	routes := map[string]string{}
	if item.ImageGenerateRoute != "" {
		routes["image_generate:"+item.ModelName] = item.ImageGenerateRoute
	}
	if item.ImageEditRoute != "" {
		routes["image_edit:"+item.ModelName] = item.ImageEditRoute
	}
	if item.VideoRoute != "" {
		routes["video:"+item.ModelName] = item.VideoRoute
	}
	if encoded, err := json.Marshal(routes); err == nil {
		cfg.ModelRoutes = string(encoded)
	}
	if strings.TrimSpace(item.VideoDurations) != "" {
		cfg.ModelVideoDurations = fmt.Sprintf(`{"%s":%s}`, item.ModelName, item.VideoDurations)
	}
	if item.VideoCustomizable {
		cfg.ModelVideoCustomizable = fmt.Sprintf(`{"%s":true}`, item.ModelName)
	}
	return cfg
}

func resolveGenerationByChannelModel(selection ChannelSelection, modelName string, modelRepo interface {
	FindByID(uint) (*model.ChannelModel, error)
}) string {
	if selection.ChannelModelID == 0 || modelRepo == nil {
		return ""
	}
	item, err := modelRepo.FindByID(selection.ChannelModelID)
	if err != nil || strings.TrimSpace(item.ModelName) != strings.TrimSpace(modelName) {
		return ""
	}
	capabilities := parseChannelCapabilities(item.Capabilities)
	if len(capabilities) == 1 {
		return capabilities[0]
	}
	return ""
}

func buildModelTestRequest(cfg *model.TenantApiConfig, input ModelTestInput) (modelTestRequest, error) {
	modelName := strings.TrimSpace(input.Model)
	generation := strings.TrimSpace(input.Generation)
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		prompt = defaultModelTestPrompt(generation)
	}

	switch generation {
	case "image":
		return buildImageModelTestRequest(cfg, input, prompt)
	case "video":
		return buildVideoModelTestRequest(cfg, input, prompt)
	case "audio":
		return jsonModelTestRequest("audio", "openai", "/audio/speech", map[string]interface{}{
			"model":           modelName,
			"input":           prompt,
			"voice":           "alloy",
			"response_format": "mp3",
			"speed":           1,
		})
	case "text":
		return jsonModelTestRequest("text", "chat", "/chat/completions", map[string]interface{}{
			"model": modelName,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
		})
	default:
		return modelTestRequest{}, errors.New("请选择要测试的模型能力")
	}
}

func buildImageModelTestRequest(cfg *model.TenantApiConfig, input ModelTestInput, prompt string) (modelTestRequest, error) {
	modelName := strings.TrimSpace(input.Model)
	route := strings.TrimSpace(input.Route)
	route = effectiveModelRoute(cfg, "image_generate", modelName, route)
	if route == "" || route == "auto" {
		if isBananaModelName(modelName) {
			route = "banana"
		} else {
			route = "generations"
		}
	}
	size := strings.TrimSpace(input.Size)
	if size == "" {
		size = "1024x1024"
	}
	hasReferences := input.HasReferences || input.ReferenceCount > 0 || strings.TrimSpace(input.Operation) == "image_edit"
	switch route {
	case "generations":
		payload := map[string]interface{}{
			"model":           modelName,
			"prompt":          prompt,
			"n":               1,
			"size":            size,
			"response_format": "b64_json",
			"output_format":   "png",
		}
		if hasReferences {
			payload["image"] = []string{testReferenceImageURL}
		}
		return jsonModelTestRequest("image", route, "/images/generations", payload)
	case "chat":
		content := []map[string]interface{}{{"type": "text", "text": prompt}}
		if hasReferences {
			content = append(content, map[string]interface{}{"type": "image_url", "image_url": map[string]string{"url": testReferenceImageURL}})
		}
		return jsonModelTestRequest("image", route, "/chat/completions", map[string]interface{}{
			"model": modelName,
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": content,
				},
			},
		})
	case "banana":
		content := interface{}(prompt)
		if hasReferences {
			content = []map[string]interface{}{
				{"type": "text", "text": prompt},
				{"type": "image_url", "image_url": map[string]string{"url": testReferenceImageURL}},
			}
		}
		return jsonModelTestRequest("image", route, "/chat/completions", map[string]interface{}{
			"model": modelName,
			"messages": []map[string]interface{}{
				{"role": "user", "content": content},
			},
			"extra_body": map[string]interface{}{
				"google": map[string]interface{}{
					"image_config": map[string]string{
						"aspect_ratio": "1:1",
						"image_size":   "1K",
					},
				},
			},
		})
	case "edits":
		return imageEditModelTestRequest(route, modelName, prompt, size)
	default:
		return modelTestRequest{}, fmt.Errorf("不支持的图片路由：%s", route)
	}
}

func imageEditModelTestRequest(route, modelName, prompt, size string) (modelTestRequest, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	imagePart, err := writer.CreateFormFile("image", "reference.png")
	if err != nil {
		return modelTestRequest{}, err
	}
	if _, err := imagePart.Write(testReferencePNG); err != nil {
		return modelTestRequest{}, err
	}
	_ = writer.WriteField("model", modelName)
	_ = writer.WriteField("prompt", prompt)
	_ = writer.WriteField("n", "1")
	_ = writer.WriteField("size", size)
	if err := writer.Close(); err != nil {
		return modelTestRequest{}, err
	}
	return modelTestRequest{Generation: "image", Route: route, Method: http.MethodPost, Path: "/images/edits", ContentType: writer.FormDataContentType(), Body: buffer.Bytes()}, nil
}

func buildVideoModelTestRequest(cfg *model.TenantApiConfig, input ModelTestInput, prompt string) (modelTestRequest, error) {
	modelName := strings.TrimSpace(input.Model)
	route := strings.TrimSpace(input.Route)
	route = effectiveModelRoute(cfg, "video", modelName, route)
	if route == "" || route == "auto" {
		route = "openai"
	}
	seconds := testVideoDuration(cfg, modelName)
	if input.Seconds > 0 {
		seconds = input.Seconds
	}
	if strings.TrimSpace(modelName) == "veo-omni-flash" {
		seconds = 10
	}
	size := strings.TrimSpace(input.Size)
	if size == "" || strings.Contains(size, "p") {
		size = "1280x720"
	}
	aspectRatio := strings.TrimSpace(input.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = aspectRatioFromSize(size)
	}
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}
	hasReferences := input.HasReferences || input.ReferenceCount > 0 || strings.TrimSpace(input.Operation) == "image_to_video" || strings.TrimSpace(input.Operation) == "video_to_video"
	switch route {
	case "openai":
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		_ = writer.WriteField("model", modelName)
		_ = writer.WriteField("prompt", prompt)
		_ = writer.WriteField("seconds", fmt.Sprintf("%d", seconds))
		_ = writer.WriteField("size", size)
		_ = writer.WriteField("resolution_name", "720p")
		_ = writer.WriteField("preset", "normal")
		if hasReferences {
			_ = writer.WriteField("image", testReferenceImageURL)
			_ = writer.WriteField("first_image_url", testReferenceImageURL)
		}
		if err := writer.Close(); err != nil {
			return modelTestRequest{}, err
		}
		return modelTestRequest{Generation: "video", Route: route, Method: http.MethodPost, Path: "/videos", ContentType: writer.FormDataContentType(), Body: buffer.Bytes()}, nil
	case "veo_json":
		payload := map[string]interface{}{
			"model":        modelName,
			"prompt":       prompt,
			"duration":     seconds,
			"aspect_ratio": aspectRatio,
		}
		if hasReferences {
			payload["input_reference"] = testReferenceImageURL
			payload["first_image"] = testReferenceImageURL
			payload["first_image_url"] = testReferenceImageURL
		}
		return jsonModelTestRequest("video", route, "/videos", payload)
	case "waninter":
		payload := map[string]interface{}{
			"model":        modelName,
			"prompt":       prompt,
			"seconds":      fmt.Sprintf("%d", seconds),
			"duration":     seconds,
			"size":         size,
			"aspect_ratio": aspectRatio,
			"resolution":   "720p",
		}
		if hasReferences {
			if isWaninterVeoStyleModelName(modelName) {
				payload["Ingredients_images"] = []string{testReferenceImageURL}
			} else {
				payload["images"] = []string{testReferenceImageURL}
			}
		}
		return jsonModelTestRequest("video", route, "/videos", payload)
	case "yijia":
		payload := map[string]interface{}{
			"model":      modelName,
			"prompt":     prompt,
			"size":       size,
			"seconds":    fmt.Sprintf("%d", seconds),
			"n":          1,
			"watermark":  false,
			"private":    false,
			"storyboard": false,
		}
		if hasReferences {
			payload["input_reference"] = testReferenceImageURL
			payload["first_image_url"] = testReferenceImageURL
		}
		return jsonModelTestRequest("video", route, "/videos", payload)
	case "xai":
		payload := map[string]interface{}{
			"model":        modelName,
			"prompt":       prompt,
			"duration":     seconds,
			"aspect_ratio": aspectRatio,
			"resolution":   "720p",
		}
		if hasReferences {
			payload["image"] = testReferenceImageURL
			payload["first_image"] = testReferenceImageURL
		}
		return jsonModelTestRequest("video", route, "/videos/generations", payload)
	case "newapi":
		payload := map[string]interface{}{
			"model":    modelName,
			"prompt":   prompt,
			"duration": seconds,
			"width":    1280,
			"height":   720,
		}
		if sizeWidth, sizeHeight := parseSize(size); sizeWidth > 0 && sizeHeight > 0 {
			payload["width"] = sizeWidth
			payload["height"] = sizeHeight
		}
		if hasReferences {
			payload["image"] = testReferenceImageURL
		}
		return jsonModelTestRequest("video", route, "/video/generations", payload)
	case "seedance":
		content := []map[string]string{{"type": "text", "text": prompt}}
		if hasReferences {
			content = append(content, map[string]string{"type": "image_url", "image_url": testReferenceImageURL})
		}
		return jsonModelTestRequest("video", route, "/contents/generations/tasks", map[string]interface{}{
			"model":          modelName,
			"content":        content,
			"ratio":          aspectRatio,
			"resolution":     "720p",
			"duration":       seconds,
			"generate_audio": true,
			"watermark":      false,
		})
	default:
		return modelTestRequest{}, fmt.Errorf("不支持的视频路由：%s", route)
	}
}

func jsonModelTestRequest(generation, route, path string, payload map[string]interface{}) (modelTestRequest, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return modelTestRequest{}, err
	}
	return modelTestRequest{Generation: generation, Route: route, Method: http.MethodPost, Path: path, ContentType: "application/json", Body: body}, nil
}

func parseSize(size string) (int, int) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return 0, 0
	}
	width, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil {
		return 0, 0
	}
	return width, height
}

func effectiveModelRoute(cfg *model.TenantApiConfig, capability, modelName, override string) string {
	if override != "" && override != "auto" {
		return override
	}
	routes := decodeModelRouteMap(cfg.ModelRoutes)
	if route := routes[capability+":"+modelName]; route != "" {
		return route
	}
	return "auto"
}

func decodeModelRouteMap(raw string) map[string]string {
	var routes map[string]string
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &routes) != nil || routes == nil {
		return map[string]string{}
	}
	return routes
}

func testVideoDuration(cfg *model.TenantApiConfig, modelName string) int {
	var items map[string][]int
	if json.Unmarshal([]byte(cfg.ModelVideoDurations), &items) == nil {
		for _, value := range items[modelName] {
			if value > 0 {
				return value
			}
		}
	}
	if strings.Contains(strings.ToLower(modelName), "veo") {
		return 10
	}
	return 5
}

func defaultModelTestPrompt(generation string) string {
	switch generation {
	case "image":
		return "生成一张用于模型连通性测试的简洁图片：一只小猫坐在白色桌面上"
	case "video":
		return "模型连通性测试：一只小猫在桌面上轻轻转头"
	case "audio":
		return "这是一段音频模型连通性测试。"
	default:
		return "请回复：模型连通性测试成功。"
	}
}

func isBananaModelName(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	return strings.Contains(name, "banana") || strings.Contains(name, "nano_banana")
}

func isWaninterVeoStyleModelName(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	return strings.Contains(name, "veo") || strings.Contains(name, "omni")
}

func responseSnippet(contentType string, body []byte) string {
	contentType = strings.ToLower(contentType)
	if !strings.Contains(contentType, "json") && !strings.Contains(contentType, "text") && !strings.Contains(contentType, "html") && len(body) > 0 {
		return fmt.Sprintf("[binary %s, %d bytes]", strings.TrimSpace(contentType), len(body))
	}
	text := strings.TrimSpace(string(body))
	return truncateString(text, 4000)
}
