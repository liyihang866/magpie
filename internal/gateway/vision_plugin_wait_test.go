package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

// A description asked of a plugin provider is a fetch the gateway does not
// write itself: plugin.Fetch hands the frame to the host. Before it said the
// frame went out, sideDeadline's per-try limit never started for a plugin
// describer, so the whole wait (visionWaitMost) held the turn; a queued
// plugin vendor outlived the agent's timeout. Here the vendor never answers:
// the description must end at the try's own limit, not at the whole wait.
func TestPluginDescriberKeepsItsTryLimit(t *testing.T) {
	t.Setenv("FAKE_SEES", "1")
	pid := besideFake(t, "fakeco", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second): // the fake vendor's connection ends with the test
		}
	}))
	up := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(up.Close)
	if err := provider.Save(provider.Provider{ID: "probe", Name: "Probe", Chat: up.URL + "/v1", Key: "key", Models: []string{"text"}}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.SaveLive("probe", up.URL+"/v1", []catalog.Model{{ID: "text", ImageInput: imageInputBool(false)}}); err != nil {
		t.Fatal(err)
	}
	pickVision(t, pid+"/fake-sees")
	visionLimits(t, 200*time.Millisecond, 10*time.Second)
	s := New()
	code, body := postAs(t, s, "", `{"model":"probe/text","messages":[{"role":"user","content":[{"type":"text","text":"read"},{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}}]}]}`)
	if code != 502 || !strings.Contains(body, "no answer in 200ms") {
		t.Fatalf("plugin describer: %d %s, want the try's own 200ms limit", code, body)
	}
}
