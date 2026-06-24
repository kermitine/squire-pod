package webserver

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

type uploadTestFile struct {
	name string
	data []byte
}

func imageUploadRequest(t *testing.T, files []uploadTestFile) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		part, err := writer.CreateFormFile("files", file.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(file.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/upload_productivity_images", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestProductivityImageLibraryLifecycle(t *testing.T) {
	oldPath := ProductivityImgPath
	oldManualConfig := vars.APIConfig.Productivity.ManualConfig
	ProductivityImgPath = t.TempDir()
	vars.APIConfig.Productivity.ManualConfig = "[]"
	t.Cleanup(func() {
		ProductivityImgPath = oldPath
		vars.APIConfig.Productivity.ManualConfig = oldManualConfig
	})

	for _, expectedName := range []string{"Morning-Face.png", "Morning-Face-2.png"} {
		response := httptest.NewRecorder()
		handleUploadProductivityImages(response, imageUploadRequest(t, []uploadTestFile{{name: "Morning Face.PNG", data: testPNG(t)}}))
		if response.Code != http.StatusCreated {
			t.Fatalf("upload status = %d, body = %q", response.Code, response.Body.String())
		}
		var uploaded []productivityImageInfo
		if err := json.NewDecoder(response.Body).Decode(&uploaded); err != nil {
			t.Fatal(err)
		}
		if len(uploaded) != 1 || uploaded[0].Name != expectedName {
			t.Fatalf("uploaded = %#v, want name %q", uploaded, expectedName)
		}
	}

	vars.APIConfig.Productivity.ManualConfig = `[{"id":"wake_up","image":"Morning-Face.png"}]`
	listResponse := httptest.NewRecorder()
	handleGetProductivityImages(listResponse)
	var images []productivityImageInfo
	if err := json.NewDecoder(listResponse.Body).Decode(&images); err != nil {
		t.Fatal(err)
	}
	var usedBy []string
	for _, image := range images {
		if image.Name == "Morning-Face.png" {
			usedBy = image.UsedBy
		}
	}
	if len(images) != 2 || len(usedBy) != 1 || usedBy[0] != "wake_up" {
		t.Fatalf("images = %#v", images)
	}

	conflictResponse := httptest.NewRecorder()
	conflictRequest := httptest.NewRequest(http.MethodDelete, "/api/delete_productivity_image", strings.NewReader(`{"name":"Morning-Face.png"}`))
	handleDeleteProductivityImage(conflictResponse, conflictRequest)
	if conflictResponse.Code != http.StatusConflict {
		t.Fatalf("referenced delete status = %d, want %d", conflictResponse.Code, http.StatusConflict)
	}

	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/delete_productivity_image", strings.NewReader(`{"name":"Morning-Face-2.png"}`))
	handleDeleteProductivityImage(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %q", deleteResponse.Code, deleteResponse.Body.String())
	}
	if _, err := os.Stat(filepath.Join(ProductivityImgPath, "Morning-Face-2.png")); !os.IsNotExist(err) {
		t.Fatalf("deleted image still exists: %v", err)
	}
}

func TestProductivityImageUploadRejectsInvalidFileWithoutPartialSave(t *testing.T) {
	oldPath := ProductivityImgPath
	ProductivityImgPath = t.TempDir()
	t.Cleanup(func() { ProductivityImgPath = oldPath })

	response := httptest.NewRecorder()
	request := imageUploadRequest(t, []uploadTestFile{
		{name: "valid.png", data: testPNG(t)},
		{name: "bad.png", data: []byte("not an image")},
	})
	handleUploadProductivityImages(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("upload status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	entries, err := os.ReadDir(ProductivityImgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial upload left %d file(s)", len(entries))
	}
}
