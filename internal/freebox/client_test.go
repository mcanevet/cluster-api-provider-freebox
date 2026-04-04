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

package freebox_test

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcanevet/cluster-api-provider-freebox/internal/freebox"
)

// challengeHandler returns a /login challenge response.
func challengeHandler(challenge string, success bool, errCode, msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type result struct {
			Challenge string `json:"challenge"`
		}
		resp := struct {
			Success   bool   `json:"success"`
			ErrorCode string `json:"error_code,omitempty"`
			Msg       string `json:"msg,omitempty"`
			Result    result `json:"result,omitempty"`
		}{
			Success:   success,
			ErrorCode: errCode,
			Msg:       msg,
			Result:    result{Challenge: challenge},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// sessionHandler returns a /login/session response.
func sessionHandler(token string, success bool, errCode, msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type result struct {
			SessionToken string `json:"session_token"`
		}
		resp := struct {
			Success   bool   `json:"success"`
			ErrorCode string `json:"error_code,omitempty"`
			Msg       string `json:"msg,omitempty"`
			Result    result `json:"result,omitempty"`
		}{
			Success:   success,
			ErrorCode: errCode,
			Msg:       msg,
			Result:    result{SessionToken: token},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func TestGetSessionToken_Success(t *testing.T) {
	const (
		appID        = "myapp"
		privateToken = "mysecret"
		challenge    = "testchallenge"
		wantToken    = "session-abc"
		version      = "v10"
	)

	// Pre-compute expected password so we can verify it in the session handler
	//nolint:gosec
	h := hmac.New(sha1.New, []byte(privateToken))
	h.Write([]byte(challenge))
	expectedPassword := hex.EncodeToString(h.Sum(nil))

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/%s/login", version), challengeHandler(challenge, true, "", ""))
	mux.HandleFunc(fmt.Sprintf("/api/%s/login/session", version), func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AppID    string `json:"app_id"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if body.AppID != appID {
			t.Errorf("got app_id=%q, want %q", body.AppID, appID)
		}
		if body.Password != expectedPassword {
			t.Errorf("got password=%q, want %q", body.Password, expectedPassword)
		}
		sessionHandler(wantToken, true, "", "")(w, r)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	got, err := freebox.GetSessionToken(srv.URL, version, appID, privateToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wantToken {
		t.Errorf("got token=%q, want %q", got, wantToken)
	}
}

func TestGetSessionToken_ChallengeFailure(t *testing.T) {
	const version = "v10"

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/%s/login", version),
		challengeHandler("", false, "auth_required", "not logged in"))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := freebox.GetSessionToken(srv.URL, version, "app", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetSessionToken_SessionFailure(t *testing.T) {
	const version = "v10"

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/%s/login", version), challengeHandler("chal", true, "", ""))
	mux.HandleFunc(fmt.Sprintf("/api/%s/login/session", version),
		sessionHandler("", false, "invalid_token", "bad credentials"))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := freebox.GetSessionToken(srv.URL, version, "app", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetDownloadDir_Success(t *testing.T) {
	const (
		version = "v10"
		rawPath = "/Disque 1/Téléchargements"
		token   = "sess-tok"
	)
	encoded := base64.StdEncoding.EncodeToString([]byte(rawPath))

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/%s/downloads/config/", version), func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Fbx-App-Auth") != token {
			t.Errorf("missing/wrong auth header: %q", r.Header.Get("X-Fbx-App-Auth"))
		}
		resp := fmt.Sprintf(`{"success":true,"result":{"download_dir":%q}}`, encoded)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	got, err := freebox.GetDownloadDir(srv.URL, version, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != rawPath {
		t.Errorf("got %q, want %q", got, rawPath)
	}
}

func TestGetDownloadDir_NotBase64(t *testing.T) {
	const version = "v10"

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/%s/downloads/config/", version), func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"result":{"download_dir":"not valid base64 !!!"}}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := freebox.GetDownloadDir(srv.URL, version, "tok")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestGetDownloadDir_APIFailure(t *testing.T) {
	const version = "v10"

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/%s/downloads/config/", version), func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error_code":"auth_required","msg":"not logged in"}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := freebox.GetDownloadDir(srv.URL, version, "tok")
	if err == nil {
		t.Fatal("expected error for API failure, got nil")
	}
}

func TestGetVMStoragePath_Success(t *testing.T) {
	const (
		version  = "v10"
		diskName = "Disque 1"
		wantPath = "/Disque 1/VMs"
		token    = "sess-tok"
	)

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/%s/system/", version), func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Fbx-App-Auth") != token {
			t.Errorf("missing/wrong auth header: %q", r.Header.Get("X-Fbx-App-Auth"))
		}
		resp := fmt.Sprintf(`{"success":true,"result":{"user_main_storage":%q}}`, diskName)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	got, err := freebox.GetVMStoragePath(srv.URL, version, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wantPath {
		t.Errorf("got %q, want %q", got, wantPath)
	}
}

func TestGetVMStoragePath_Empty(t *testing.T) {
	const version = "v10"

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/%s/system/", version), func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"result":{"user_main_storage":""}}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := freebox.GetVMStoragePath(srv.URL, version, "tok")
	if err == nil {
		t.Fatal("expected error for empty user_main_storage, got nil")
	}
}

func TestGetVMStoragePath_APIFailure(t *testing.T) {
	const version = "v10"

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/%s/system/", version), func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error_code":"auth_required","msg":"not logged in"}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := freebox.GetVMStoragePath(srv.URL, version, "tok")
	if err == nil {
		t.Fatal("expected error for API failure, got nil")
	}
}
