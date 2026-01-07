package terminal

// Terminal 统一终端接口
type Terminal interface {
	ID() string
	Type() TerminalType // local, ssh, docker

	// 生命周期
	Start() error
	Close() error

	// IO操作
	Read() ([]byte, error)
	Write(data []byte) error
	Resize(cols, rows uint16) error

	// 状态
	Status() TerminalStatus
	Metadata() map[string]interface{}
}

type TerminalType string

const (
	TerminalTypeLocal  TerminalType = "local"
	TerminalTypeSSH    TerminalType = "ssh"
	TerminalTypeDocker TerminalType = "docker"
)
