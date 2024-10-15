package audio

type DeviceType string

const (
	DeviceTypeCapture  DeviceType = "capture"
	DeviceTypePlayback DeviceType = "playback"
)

type Device struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

type Devices struct {
	Playback []Device `json:"playback"`
	Capture  []Device `json:"capture"`
}

type RecordInfo struct {
	SampleCount int `json:"sample_count"`
	DurationMs  int `json:"duration_ms"`
	EncodedSize int `json:"encoded_size"`
	PacketCount int `json:"packet_count"`
}
