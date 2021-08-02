package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// const AppRoot = "/opt/xmppeeker"
const AppRoot string = "."

const (
	DefaultCertificate     string = "xmppeeker.crt"
	DefaultCertificateKey  string = "xmppeeker.key"
	DefaultCertificatePath string = "certs"
	DefaultLogPath         string = "logs"
)
const (
	ExitOK int = iota
	ExitBadConfig
	ExitFatal
)

func handleConnection(logger *zap.SugaredLogger, c net.Conn, config *ProxyConfig) {
	p := NewProxy(c, config)
	err := p.Run()
	if err != nil {
		logger.Errorw("error while running proxy",
			"reason", err.Error(),
			"clientAddr", c.RemoteAddr().String(),
			"serverAddr", config.Address,
		)
	}
}

func main() {
	c := zap.NewProductionConfig()
	c.EncoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	logger, _ := c.Build()
	sugar := logger.Sugar()
	defer logger.Sync()

	configureViper(sugar)
	pConfig := createProxyConfig(sugar)

	listenAddr := fmt.Sprintf("%s:%s", viper.GetString("ListenHost"), viper.GetString("ListenPort"))
	listener, err := net.Listen("tcp4", listenAddr)
	if err != nil {
		sugar.Errorw("failed to start listener",
			"reason", err.Error(),
		)
		os.Exit(ExitFatal)
	}
	defer listener.Close()

	sugar.Infow("xmppeeker started",
		"ListenHost", viper.GetString("ListenHost"),
		"ListenPort", viper.GetString("ListenPort"),
		"BackendHost", viper.GetString("BackendHost"),
		"BackendPort", viper.GetString("BackendPort"),
	)

	// Main loop
	for {
		c, err := listener.Accept()
		if err != nil {
			sugar.Errorw("error accepting connection",
				"reason", err.Error(),
			)
			return
		}
		// TODO: Limit the number of goroutines spawned instead of infinitely creating them.
		go handleConnection(sugar, c, pConfig)
	}
}

func configureViper(sugar *zap.SugaredLogger) {
	viper.SetConfigName("xmppeeker")
	viper.SetConfigType("toml")
	viper.AddConfigPath(filepath.Join(AppRoot, "conf"))

	viper.SetDefault("BackendPort", 5222)
	viper.SetDefault("ListenHost", "0.0.0.0")
	viper.SetDefault("ListenPort", 5222)
	viper.SetDefault("ConnectTimeout", 10)
	viper.SetDefault("LogTimeFormat", "2006-01-02 15:04:05.000000")
	viper.SetDefault("FileTimeFormat", "2006-01-02_15-04-05")
	viper.SetDefault("Certificate", filepath.Join(DefaultCertificatePath, DefaultCertificate))
	viper.SetDefault("CertificateKey", filepath.Join(DefaultCertificatePath, DefaultCertificateKey))
	viper.SetDefault("LogPath", DefaultLogPath)

	err := viper.ReadInConfig()

	if err != nil {
		sugar.Errorw("failed to load config",
			"reason", err.Error(),
		)
		os.Exit(ExitBadConfig)
	}

	// Override loaded conf with ENV variables
	viper.SetEnvPrefix("PEEKER")
	viper.AutomaticEnv()

	// BackendHost is a required field
	if beHost := viper.GetString("BackendHost"); !validator.IsAddress(beHost) {
		sugar.Errorw("failed to load config",
			"reason", "'BackendHost' is invalid. must be either an IP address or hostname",
			"value", beHost,
		)
		os.Exit(ExitBadConfig)
	}

	logPath := viper.GetString("LogPath")
	if !filepath.IsAbs(logPath) {
		logPath, err = filepath.Abs(filepath.Join(AppRoot, viper.GetString("LogPath")))
		if err != nil {
			sugar.Warnw("bad log path",
				"reason", err.Error(),
			)
			os.Exit(ExitBadConfig)
		}
		viper.Set("LogPath", logPath)
	}

	certPath := viper.GetString("Certificate")
	if !filepath.IsAbs(certPath) {
		certPath, err = filepath.Abs(filepath.Join(AppRoot, viper.GetString("Certificate")))
		if err != nil {
			sugar.Warnw("bad certificate file path",
				"reason", err.Error(),
			)
		}
		viper.Set("Certificate", certPath)
	}
	keyPath := viper.GetString("CertificateKey")
	if !filepath.IsAbs(keyPath) {
		keyPath, err = filepath.Abs(filepath.Join(AppRoot, viper.GetString("CertificateKey")))
		if err != nil {
			sugar.Warnw("bad key file path",
				"reason", err.Error(),
			)
		}
		viper.Set("CertificateKey", keyPath)
	}
}

func createProxyConfig(sugar *zap.SugaredLogger) *ProxyConfig {
	cert, err := tls.LoadX509KeyPair(viper.GetString("Certificate"), viper.GetString("CertificateKey"))
	if err != nil {
		sugar.Warnw("failed to load x509 key pair",
			"reason", err.Error(),
			"certificate", viper.GetString("Certificate"),
			"key", viper.GetString("CertificateKey"),
		)

		if err := os.MkdirAll(filepath.Join(AppRoot, DefaultCertificatePath), 0755); err != nil {
			sugar.Errorw("failed to certs directory",
				"reason", err.Error(),
			)
			os.Exit(ExitBadConfig)
		}
		cert, err = generateAndSaveSelfSignedCert(sugar)
		if err != nil {
			sugar.Errorw("failed to generate self-signed certificate",
				"reason", err.Error(),
			)
		}
	}

	pConfig := &ProxyConfig{
		Address:        fmt.Sprintf("%s:%s", viper.GetString("BackendHost"), viper.GetString("BackendPort")),
		Domain:         viper.GetString("BackendHost"),
		ConnectTimeout: viper.GetInt("ConnectTimeout"),
		LogPath:        viper.GetString("LogPath"),
		LogTimeFormat:  viper.GetString("LogTimeFormat"),
		FileTimeFormat: viper.GetString("FileTimeFormat"),
		TLSConfig:      &tls.Config{Certificates: []tls.Certificate{cert}},
	}
	return pConfig
}
