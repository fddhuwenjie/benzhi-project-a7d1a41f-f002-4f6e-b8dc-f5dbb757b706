package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"quarantine-workbench/internal/policy"
)

type movingClock struct{ current time.Time }

func (c *movingClock) Now() time.Time { return c.current }

type selfResponse struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
type caseResult struct {
	Data struct {
		Case struct {
			ID       string `json:"id"`
			Revision int64  `json:"revision"`
			Status   string `json:"status"`
		} `json:"case"`
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
		Status   string `json:"status"`
	} `json:"data"`
}

func runSelfCheck(cfg config) (runErr error) {
	tmp, err := os.CreateTemp("", "quarantine-self-check-*.db")
	if err != nil {
		return err
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)
	cfg.Database = path
	clock := &movingClock{current: time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)}
	ctx := context.Background()
	app, err := buildApplication(ctx, cfg, clock)
	if err != nil {
		return err
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- app.serve() }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		runErr = combine(runErr, app.shutdown(shutdownCtx))
		select {
		case err := <-serveErr:
			runErr = combine(runErr, err)
		default:
		}
	}()
	base := "http://" + app.listener.Addr().String()
	client := &http.Client{Timeout: 5 * time.Second}
	if err = waitHealthy(client, base); err != nil {
		return err
	}
	caseID, revision, err := selfCreate(client, base)
	if err != nil {
		return err
	}
	revision, err = selfUpdateCase(client, base, caseID, revision)
	if err != nil {
		return err
	}
	riskFields := map[string]any{"spread_pathways": []string{"种子"}, "potential_hosts": []string{"蔷薇科植物"}, "source_confidence": "high", "quarantine_days": 1, "observation_interval_days": 1, "release_conditions": []string{"无病虫征象"}}
	trialToken, err := selfRiskTrial(client, base, caseID, revision, riskFields)
	if err != nil {
		return err
	}
	riskFields["trial_token"] = trialToken
	revision, err = selfMutate(client, base, caseID, "risk", revision, "manager", riskFields)
	if err != nil {
		return err
	}
	revision, err = selfMutate(client, base, caseID, "review", revision, "reviewer", map[string]any{"approved": true, "reason": "隔离方案和证据要求完整"})
	if err != nil {
		return err
	}
	revision, err = selfMutate(client, base, caseID, "start", revision, "reviewer", nil)
	if err != nil {
		return err
	}
	revision, err = selfMutate(client, base, caseID, "observations", revision, "observer", map[string]any{"observed_on": "2026-08-01", "growth_condition": "长势稳定", "pest_signs": "无", "reproduction_signs": "无", "sample_reference": "SC-001", "notes": "启动日观察"})
	if err != nil {
		return err
	}
	clock.current = clock.current.AddDate(0, 0, 1)
	revision, err = selfMutate(client, base, caseID, "observations", revision, "observer", map[string]any{"observed_on": "2026-08-02", "growth_condition": "长势稳定", "pest_signs": "无", "reproduction_signs": "无", "sample_reference": "SC-002", "notes": "期满观察"})
	if err != nil {
		return err
	}
	revision, err = selfMutate(client, base, caseID, "eligibility-check", revision, "reviewer", nil)
	if err != nil {
		return err
	}
	revision, err = selfMutate(client, base, caseID, "decision", revision, "reviewer", map[string]any{"outcome": "release", "rationale": "隔离期满、观察连续且证据完整"})
	if err != nil {
		return err
	}
	var detail struct {
		Data struct {
			Case struct {
				Status   string `json:"status"`
				Revision int64  `json:"revision"`
			} `json:"case"`
		} `json:"data"`
		Timeline []any `json:"timeline"`
	}
	if err = getJSON(client, base+"/api/cases/"+caseID, &detail); err != nil {
		return err
	}
	if detail.Data.Case.Status != "released" || detail.Data.Case.Revision != revision || len(detail.Timeline) != 9 {
		return fmt.Errorf("自检归档结果不完整：status=%s revision=%d events=%d", detail.Data.Case.Status, detail.Data.Case.Revision, len(detail.Timeline))
	}
	fmt.Printf("自检通过：个案 %s 已放行归档，revision=%d，审计事件=%d\n", caseID, revision, len(detail.Timeline))
	return nil
}

func selfRiskTrial(client *http.Client, base, caseID string, revision int64, fields map[string]any) (string, error) {
	body := map[string]any{"expected_revision": revision, "request_id": "self-risk-trial", "actor": "自检用户", "role": "manager"}
	for k, v := range fields {
		body[k] = v
	}
	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := postJSON(client, fmt.Sprintf("%s/api/cases/%s/risk/trial", base, caseID), body, &result); err != nil {
		return "", err
	}
	if result.Data.Token == "" {
		return "", fmt.Errorf("风险试算未返回 token")
	}
	return result.Data.Token, nil
}

func waitHealthy(client *http.Client, base string) error {
	var last error
	for i := 0; i < 30; i++ {
		var body map[string]string
		last = getJSON(client, base+"/api/health", &body)
		if last == nil && body["status"] == "ok" {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("服务健康检查失败: %w", last)
}

func selfCreate(client *http.Client, base string) (string, int64, error) {
	body := map[string]any{"request_id": "self-create", "actor": "自检管理员", "role": "manager", "accession_code": " SELF-2026-001 ", "scientific_name": "Rosa testacea", "origin_region": "", "introduction_purpose": "流程验证", "quarantine_zone": "自检隔离区"}
	var result caseResult
	if err := postJSON(client, base+"/api/cases", body, &result); err != nil {
		return "", 0, err
	}
	return result.Data.ID, result.Data.Revision, nil
}

func selfUpdateCase(client *http.Client, base, caseID string, revision int64) (int64, error) {
	body := map[string]any{"expected_revision": revision, "request_id": "self-case-update", "actor": "自检管理员", "role": "manager", "accession_code": "self-2026-001", "scientific_name": "Rosa testacea", "origin_region": "自检地区", "introduction_purpose": "流程验证", "quarantine_zone": "自检隔离区"}
	raw, _ := json.Marshal(body)
	request, err := http.NewRequest(http.MethodPatch, base+"/api/cases/"+caseID, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	var result caseResult
	if err = decodeHTTP(response, &result); err != nil {
		return 0, err
	}
	if result.Data.Case.Revision != revision+1 {
		return 0, fmt.Errorf("草稿补全后修订号未增长")
	}
	return result.Data.Case.Revision, nil
}

func selfMutate(client *http.Client, base, id, action string, revision int64, role string, extra map[string]any) (int64, error) {
	body := map[string]any{"expected_revision": revision, "request_id": fmt.Sprintf("self-%s-%d", action, revision), "actor": "自检用户", "role": role}
	for k, v := range extra {
		body[k] = v
	}
	var result caseResult
	if err := postJSON(client, fmt.Sprintf("%s/api/cases/%s/%s", base, id, action), body, &result); err != nil {
		return 0, err
	}
	if result.Data.Case.Revision != revision+1 {
		return 0, fmt.Errorf("%s 后修订号未增长", action)
	}
	return result.Data.Case.Revision, nil
}

func postJSON(client *http.Client, url string, input, output any) error {
	raw, _ := json.Marshal(input)
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return decodeHTTP(response, output)
}
func getJSON(client *http.Client, url string, output any) error {
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return decodeHTTP(response, output)
}
func decodeHTTP(response *http.Response, output any) error {
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, string(raw))
	}
	if err = json.Unmarshal(raw, output); err != nil {
		return fmt.Errorf("解析响应: %w", err)
	}
	return nil
}

var _ policy.Clock = (*movingClock)(nil)
