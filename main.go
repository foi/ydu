package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/dustin/go-humanize"
)

const yandexUploadUrl = "https://cloud-api.yandex.net/v1/disk/resources/upload"

func uploadFile(
	httpClient *http.Client,
	uploadURL, filePath string,
) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf(
			"filed to open source file: %v",
			err,
		)
	}
	defer file.Close()

	req, err := http.NewRequest(
		http.MethodPut,
		uploadURL,
		file,
	)
	if err != nil {
		return fmt.Errorf(
			"error during creating upload request: %v",
			err,
		)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"error during upload: %v",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"upload error: %s, body: %s",
			resp.Status,
			string(body),
		)
	}

	return nil
}

type UploadTarget struct {
	OperationID string `json:"operation_id"`
	Href        string `json:"href"`
	Method      string `json:"method"`
	Templated   bool   `json:"templated"`
}

func createRequestOnUpload(
	httpClient *http.Client,
	yandexDiskPath,
	token string,
) (*string, error) {

	params := url.Values{}
	params.Add("path", yandexDiskPath)

	u, err := url.Parse(yandexUploadUrl)
	if err != nil {
		return nil, err
	}

	u.RawQuery = params.Encode()

	req, err := http.NewRequest(
		http.MethodGet,
		u.String(),
		nil,
	)

	if err != nil {
		return nil, err
	}

	req.Header.Add(
		"Authorization",
		fmt.Sprintf("OAuth %s", token),
	)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	resp.Body.Close()

	var target UploadTarget

	err = json.Unmarshal(
		[]byte(body),
		&target,
	)

	if err != nil {
		return nil, err
	}

	return &target.Href, nil
}

func main() {
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, nil),
	)

	filePath := flag.String(
		"path-to-file",
		"",
		"path to source file",
	)
	yandexDiskUploadPath := flag.String(
		"target-yandex-disk-path",
		"",
		"target path on yandex disk",
	)
	httpClientTimeout := flag.Int(
		"timeout",
		900,
		"http client timeout (sec)",
	)

	token := os.Getenv("YANDEX_DISK_TOKEN")

	flag.Parse()

	if *filePath == "" ||
		*yandexDiskUploadPath == "" ||
		token == "" {
		slog.Error(
			"please set --path-to-file, --target-yandex-disk-path, and pass ENV variable with yandex disk token YANDEX_DISK_TOKEN",
		)
		os.Exit(1)
	}

	fileInfo, err := os.Stat(*filePath)
	if err != nil {
		logger.Error(
			"Error dusting checking source file existence",
			slog.String("path", *filePath),
			slog.String("message", err.Error()),
		)
		os.Exit(1)
	}

	httpClient := http.Client{
		Timeout: time.Second * time.Duration(
			*httpClientTimeout,
		),
	}

	logger.Info(
		"src file size",
		slog.String(
			"src file path",
			*filePath,
		),
		slog.String(
			"size",
			humanize.Bytes(
				uint64(fileInfo.Size()),
			),
		),
		slog.String(
			"target yandex disk path",
			*yandexDiskUploadPath,
		),
	)

	uploadUrl, err := createRequestOnUpload(
		&httpClient,
		*yandexDiskUploadPath,
		token,
	)

	if err != nil {
		logger.Error(
			"Error during create upload request to yandex disk",
			slog.String("message", err.Error()),
		)
		os.Exit(1)
	}

	logger.Info("upload url received")

	err = uploadFile(
		&httpClient,
		*uploadUrl,
		*filePath,
	)
	if err != nil {
		slog.Error(
			"Erroro during upload file",
			slog.String("message", err.Error()),
		)
		os.Exit(1)
	}

	logger.Info(
		"file uploaded successfully",
		slog.String("file", *filePath),
	)
}
