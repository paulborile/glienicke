package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNIP11_RelayInformationDocument(t *testing.T) {
	wsURL, _, cleanup := setupRelay(t)
	defer cleanup()

	httpURL := strings.Replace(wsURL, "ws://", "http://", 1)

	req, err := http.NewRequest("GET", httpURL, nil)
	assert.NoError(t, err)

	req.Header.Set("Accept", "application/nostr+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/nostr+json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var infoDoc map[string]interface{}
	err = json.Unmarshal(body, &infoDoc)
	assert.NoError(t, err)

	assert.Equal(t, "Glienicke Nostr Relay", infoDoc["name"])
	assert.Equal(t, "A Nostr relay written in Go", infoDoc["description"])
	assert.Equal(t, "https://github.com/paul/glienicke", infoDoc["software"])
	assert.Equal(t, "v0.3.0", infoDoc["version"])

	supportedNIPs, ok := infoDoc["supported_nips"].([]interface{})
	assert.True(t, ok)

	expectedNIPs := []float64{1, 9, 11}
	assert.ElementsMatch(t, expectedNIPs, supportedNIPs)
}
