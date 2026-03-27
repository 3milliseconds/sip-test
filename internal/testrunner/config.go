package testrunner

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nightowl/sip-test/internal/media"
	"gopkg.in/yaml.v3"
)

// Config represents the top-level test configuration.
type Config struct {
	Tests []TestConfig `yaml:"tests" json:"tests"`
}

// TestConfig defines a single SIP test case.
type TestConfig struct {
	Name         string        `yaml:"name" json:"name"`
	Target       string        `yaml:"target" json:"target"`               // sip:user@host:port
	From         string        `yaml:"from" json:"from"`                   // sip:tester@localip
	Transport    string        `yaml:"transport" json:"transport"`         // udp, tcp
	Codec        string        `yaml:"codec" json:"codec"`                 // PCMU, PCMA
	Duration     time.Duration `yaml:"duration" json:"duration"`           // call duration
	MediaFile    string        `yaml:"media_file" json:"media_file"`       // path to audio file
	RepeatMedia  bool          `yaml:"repeat_media" json:"repeat_media"`   // loop audio
	RTPPort      int           `yaml:"rtp_port" json:"rtp_port"`           // local RTP port (0 = auto)
	SIPPort      int           `yaml:"sip_port" json:"sip_port"`           // local SIP port (default 5060)
	Expected     Expected      `yaml:"expected" json:"expected"`
}

// Expected defines expected outcomes for test validation.
type Expected struct {
	ResponseCode int           `yaml:"response_code" json:"response_code"`
	SetupTimeMax time.Duration `yaml:"setup_time_max" json:"setup_time_max"`
}

// ParseCodec converts a codec string to CodecType.
func ParseCodec(codec string) (media.CodecType, error) {
	switch strings.ToUpper(codec) {
	case "PCMU", "G711U", "ULAW":
		return media.CodecPCMU, nil
	case "PCMA", "G711A", "ALAW":
		return media.CodecPCMA, nil
	default:
		return 0, fmt.Errorf("unsupported codec: %s", codec)
	}
}

// LoadConfig reads and parses a YAML test configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Apply defaults
	for i := range cfg.Tests {
		if cfg.Tests[i].Transport == "" {
			cfg.Tests[i].Transport = "udp"
		}
		if cfg.Tests[i].Codec == "" {
			cfg.Tests[i].Codec = "PCMU"
		}
		if cfg.Tests[i].Duration == 0 {
			cfg.Tests[i].Duration = 30 * time.Second
		}
		if cfg.Tests[i].SIPPort == 0 {
			cfg.Tests[i].SIPPort = 5060
		}
		if cfg.Tests[i].Expected.ResponseCode == 0 {
			cfg.Tests[i].Expected.ResponseCode = 200
		}
		if cfg.Tests[i].Expected.SetupTimeMax == 0 {
			cfg.Tests[i].Expected.SetupTimeMax = 10 * time.Second
		}
	}

	return &cfg, nil
}
