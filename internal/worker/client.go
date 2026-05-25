package worker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/grootpxw/edgetunnel-bestsub/internal/config"
)

type Client struct {
	baseURL   string
	password  string
	userAgent string
	http      *http.Client
}

func New(cfg config.Config) (*Client, error) {
	if cfg.Worker.BaseURL == "" {
		return nil, fmt.Errorf("worker.base_url is required")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:   strings.TrimRight(cfg.Worker.BaseURL, "/"),
		password:  cfg.Worker.Password,
		userAgent: cfg.Worker.UserAgent,
		http: &http.Client{
			Timeout: 20 * time.Second,
			Jar:     jar,
			Transport: &http.Transport{
				Proxy: nil,
			},
		},
	}, nil
}

func (c *Client) Login(ctx context.Context) error {
	if c.password == "" {
		return fmt.Errorf("worker.password is required")
	}
	form := url.Values{}
	form.Set("password", c.password)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("login returned %s", resp.Status)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if !strings.Contains(string(body), "success") {
		return fmt.Errorf("Worker 登录失败（密码可能错误）")
	}
	return nil
}

func (c *Client) PushADD(ctx context.Context, body string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/admin/ADD.txt", strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=UTF-8")
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("push returned %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}
	if !strings.Contains(string(responseBody), "success") {
		return fmt.Errorf("push did not return success: %s", strings.TrimSpace(string(responseBody)))
	}
	return nil
}

// HTTPClient 返回内部已登录的 http.Client（带 cookie），供其他模块复用会话。
func (c *Client) HTTPClient() *http.Client {
	return c.http
}

// BaseURL 返回 Worker 的基础 URL。
func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) PushProxyIP(ctx context.Context, body string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/admin/PROXYIP.txt", strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=UTF-8")
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("push proxyip returned %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}
	if !strings.Contains(string(responseBody), "success") {
		return fmt.Errorf("push proxyip did not return success: %s", strings.TrimSpace(string(responseBody)))
	}
	return nil
}
