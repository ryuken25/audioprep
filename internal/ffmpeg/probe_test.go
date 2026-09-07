package ffmpeg

import (
	"os"
	"testing"
)

func TestParseProbeJSON_RealSample(t *testing.T) {
	data, err := os.ReadFile("testdata/probe_sample.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseProbeJSON(data)
	if err != nil {
		t.Fatalf("ParseProbeJSON: %v", err)
	}
	if p.Video == nil || p.Audio == nil {
		t.Fatalf("expected both streams, got video=%v audio=%v", p.Video, p.Audio)
	}
	if p.Video.Codec != "h264" || p.Video.Width != 1280 || p.Video.Height != 720 {
		t.Errorf("video = %+v", p.Video)
	}
	if p.Video.FPS < 29.9 || p.Video.FPS > 30.1 {
		t.Errorf("fps = %v, want ~30", p.Video.FPS)
	}
	if p.Video.Rotation != 0 {
		t.Errorf("rotation = %d, want 0", p.Video.Rotation)
	}
	if p.Audio.Codec != "aac" || p.Audio.SampleRate != 44100 || p.Audio.Channels != 1 {
		t.Errorf("audio = %+v", p.Audio)
	}
	if p.Duration < 4.9 || p.Duration > 5.2 {
		t.Errorf("duration = %v, want ~5", p.Duration)
	}
	if p.Size == 0 || p.BitRate == 0 {
		t.Errorf("size=%d bitrate=%d, want non-zero", p.Size, p.BitRate)
	}
}

// A phone clip: stored landscape, display matrix says rotate -90.
const rotatedJSON = `{
  "streams": [
    {
      "codec_type": "video", "codec_name": "hevc",
      "width": 1920, "height": 1080,
      "r_frame_rate": "30000/1001", "avg_frame_rate": "29970/1000",
      "bit_rate": "12000000",
      "side_data_list": [ { "side_data_type": "Display Matrix", "rotation": -90 } ]
    },
    { "codec_type": "audio", "codec_name": "aac", "sample_rate": "48000", "channels": 2, "bit_rate": "96000" }
  ],
  "format": { "format_name": "mov,mp4,m4a,3gp,3g2,mj2", "duration": "42.5", "size": "64000000", "bit_rate": "12096000" }
}`

func TestParseProbeJSON_RotationSideData(t *testing.T) {
	p, err := ParseProbeJSON([]byte(rotatedJSON))
	if err != nil {
		t.Fatal(err)
	}
	if p.Video.Rotation != 270 {
		t.Errorf("rotation = %d, want 270 (from -90)", p.Video.Rotation)
	}
	if p.Video.DisplayWidth() != 1080 || p.Video.DisplayHeight() != 1920 {
		t.Errorf("display = %dx%d, want 1080x1920", p.Video.DisplayWidth(), p.Video.DisplayHeight())
	}
	if p.Video.FPS < 29.9 || p.Video.FPS > 30.0 {
		t.Errorf("fps = %v", p.Video.FPS)
	}
}

func TestParseProbeJSON_LegacyRotateTag(t *testing.T) {
	js := `{"streams":[{"codec_type":"video","codec_name":"h264","width":1280,"height":720,
	  "r_frame_rate":"30/1","tags":{"rotate":"90"}}],"format":{"duration":"1"}}`
	p, err := ParseProbeJSON([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	if p.Video.Rotation != 90 {
		t.Errorf("rotation = %d, want 90", p.Video.Rotation)
	}
}

func TestParseProbeJSON_SkipsCoverArt(t *testing.T) {
	// An MP3 with embedded album art: ffprobe lists a "video" stream that
	// is really a JPEG with attached_pic=1. It must not count as video.
	js := `{"streams":[
	  {"codec_type":"audio","codec_name":"mp3","sample_rate":"44100","channels":2,"bit_rate":"320000"},
	  {"codec_type":"video","codec_name":"mjpeg","width":600,"height":600,"disposition":{"attached_pic":1}}
	],"format":{"format_name":"mp3","duration":"200.1"}}`
	p, err := ParseProbeJSON([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	if p.HasVideo() {
		t.Errorf("cover art was treated as a video stream: %+v", p.Video)
	}
	if p.Audio == nil || p.Audio.Codec != "mp3" {
		t.Errorf("audio = %+v", p.Audio)
	}
}

func TestParseRate(t *testing.T) {
	cases := map[string]float64{
		"30/1":       30,
		"30000/1001": 29.97,
		"60":         60,
		"0/0":        0,
		"":           0,
		"abc":        0,
	}
	for in, want := range cases {
		got := parseRate(in)
		if diff := got - want; diff > 0.01 || diff < -0.01 {
			t.Errorf("parseRate(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseRotation_Normalises(t *testing.T) {
	for _, tc := range []struct {
		deg  float64
		want int
	}{
		{0, 0}, {90, 90}, {-90, 270}, {180, 180}, {-180, 180}, {270, 270}, {-270, 90}, {360, 0}, {89.9, 90},
	} {
		s := rawStream{SideData: []rawSideData{{Type: "Display Matrix", Rotation: tc.deg}}}
		if got := parseRotation(s); got != tc.want {
			t.Errorf("rotation %v -> %d, want %d", tc.deg, got, tc.want)
		}
	}
}
