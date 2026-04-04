/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package freebox provides helpers for direct Freebox HTTP API calls that are
// not yet exposed by the free-go client library.
package freebox

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// GetSessionToken opens a Freebox API session and returns the session token.
// It performs the two-step HMAC-SHA1 challenge/response login flow.
// This is needed because the free-go library does not expose all endpoints.
func GetSessionToken(endpoint, version, appID, privateToken string) (string, error) {
	// Step 1: Get the login challenge
	challengeURL := fmt.Sprintf("%s/api/%s/login", endpoint, version)
	resp, err := httpClient.Get(challengeURL)
	if err != nil {
		return "", fmt.Errorf("failed to get login challenge: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read challenge response: %w", err)
	}

	var challengeResult struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"error_code,omitempty"`
		Msg       string `json:"msg,omitempty"`
		Result    struct {
			Challenge string `json:"challenge"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &challengeResult); err != nil {
		return "", fmt.Errorf("failed to parse challenge response: %w", err)
	}

	if !challengeResult.Success {
		if challengeResult.ErrorCode != "" || challengeResult.Msg != "" {
			return "", fmt.Errorf(
				"challenge request failed: error_code=%s, msg=%s",
				challengeResult.ErrorCode,
				challengeResult.Msg,
			)
		}
		return "", fmt.Errorf("challenge request was not successful")
	}

	// Step 2: Compute the password (HMAC-SHA1 of challenge with private token)
	//nolint:gosec // SHA1 is required by Freebox API
	h := hmac.New(sha1.New, []byte(privateToken))
	h.Write([]byte(challengeResult.Result.Challenge))
	password := hex.EncodeToString(h.Sum(nil))

	// Step 3: Open a session
	sessionURL := fmt.Sprintf("%s/api/%s/login/session", endpoint, version)
	sessionPayload := fmt.Sprintf(`{"app_id":"%s","password":"%s"}`, appID, password)

	sessionResp, err := httpClient.Post(sessionURL, "application/json", strings.NewReader(sessionPayload))
	if err != nil {
		return "", fmt.Errorf("failed to open session: %w", err)
	}
	defer func() { _ = sessionResp.Body.Close() }()

	sessionBody, err := io.ReadAll(sessionResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read session response: %w", err)
	}

	var sessionResult struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"error_code,omitempty"`
		Msg       string `json:"msg,omitempty"`
		Result    struct {
			SessionToken string `json:"session_token"`
		} `json:"result"`
	}

	if err := json.Unmarshal(sessionBody, &sessionResult); err != nil {
		return "", fmt.Errorf("failed to parse session response: %w", err)
	}

	if !sessionResult.Success {
		if sessionResult.ErrorCode != "" || sessionResult.Msg != "" {
			return "", fmt.Errorf(
				"session request failed: error_code=%s, msg=%s",
				sessionResult.ErrorCode,
				sessionResult.Msg,
			)
		}
		return "", fmt.Errorf("session request was not successful")
	}

	return sessionResult.Result.SessionToken, nil
}

// GetDownloadDir queries the Freebox /downloads/config/ endpoint and returns
// the decoded download directory path.
// This is a direct HTTP call since the free-go library does not expose the
// /downloads/config/ endpoint yet.
func GetDownloadDir(endpoint, version, sessionToken string) (string, error) {
	configURL := fmt.Sprintf("%s/api/%s/downloads/config/", endpoint, version)

	req, err := http.NewRequest("GET", configURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Fbx-App-Auth", sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"error_code,omitempty"`
		Msg       string `json:"msg,omitempty"`
		Result    struct {
			DownloadDir string `json:"download_dir"` // Base64-encoded path
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if !result.Success {
		if result.ErrorCode != "" || result.Msg != "" {
			return "", fmt.Errorf("API call failed: error_code=%s, msg=%s", result.ErrorCode, result.Msg)
		}
		return "", fmt.Errorf("API call was not successful (no error details provided)")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(result.Result.DownloadDir)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 download_dir: %w", err)
	}

	downloadDir := string(decodedBytes)
	if downloadDir == "" {
		return "", fmt.Errorf("download_dir is empty after decoding")
	}

	return downloadDir, nil
}

// GetVMStoragePath queries the Freebox /system/ endpoint and returns the path
// used to store VM disk images (e.g. "/Disque 1/VMs").
// This is a direct HTTP call since the free-go library does not expose the
// /system/ endpoint yet.
func GetVMStoragePath(endpoint, version, sessionToken string) (string, error) {
	systemURL := fmt.Sprintf("%s/api/%s/system/", endpoint, version)

	req, err := http.NewRequest("GET", systemURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Fbx-App-Auth", sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"error_code,omitempty"`
		Msg       string `json:"msg,omitempty"`
		Result    struct {
			UserMainStorage string `json:"user_main_storage"` // Plain string, e.g. "Disque 1"
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if !result.Success {
		if result.ErrorCode != "" || result.Msg != "" {
			return "", fmt.Errorf("API call failed: error_code=%s, msg=%s", result.ErrorCode, result.Msg)
		}
		return "", fmt.Errorf("API call was not successful (no error details provided)")
	}

	if result.Result.UserMainStorage == "" {
		return "", fmt.Errorf("user_main_storage is empty in response")
	}

	return "/" + result.Result.UserMainStorage + "/VMs", nil
}
