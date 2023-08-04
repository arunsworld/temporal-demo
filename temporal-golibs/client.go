package temporalgolibs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"time"

	"go.temporal.io/sdk/client"
)

/*
Relies on:
  - TEMPORAL_ADDRESS
  - TEMPORAL_TLS_CA
  - TEMPORAL_TLS_SERVER_NAME
  - TEMPORAL_TLS_KEY
  - TEMPORAL_TLS_CERT
*/

const clientCreationRetryDuration = time.Second * 5

func NewClient(ctx context.Context, namespace string) (client.Client, error) {
	tlsConfig, err := generateTLSConfig()
	if err != nil {
		return nil, err
	}
	for {
		c, err := tryCreatingClient(namespace, tlsConfig)
		if err == nil {
			return c, nil
		}
		log.Printf("transient error creating temporal client: %v", err)
		select {
		case <-time.After(clientCreationRetryDuration):
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled")
		}
	}
}

func tryCreatingClient(namespace string, tlsConfig *tls.Config) (client.Client, error) {
	return client.Dial(client.Options{
		HostPort:  os.Getenv("TEMPORAL_ADDRESS"),
		Namespace: namespace,
		ConnectionOptions: client.ConnectionOptions{
			TLS: tlsConfig,
		},
	})
}

func generateTLSConfig() (*tls.Config, error) {
	result := &tls.Config{}
	if os.Getenv("TEMPORAL_TLS_CA") != "" {
		caCertPool, err := generateCACertPool()
		if err != nil {
			return nil, fmt.Errorf("bad TEMPORAL_TLS_CA configuration: %w", err)
		}
		result.RootCAs = caCertPool
	}
	if os.Getenv("TEMPORAL_TLS_KEY") != "" && os.Getenv("TEMPORAL_TLS_CERT") != "" {
		tlsCert, err := generateTLSCert()
		if err != nil {
			return nil, fmt.Errorf("bad TEMPORAL_TLS_KEY and TEMPORAL_TLS_CERT configuration: %w", err)
		}
		result.Certificates = []tls.Certificate{tlsCert}
	}
	if os.Getenv("TEMPORAL_TLS_SERVER_NAME") != "" {
		result.ServerName = os.Getenv("TEMPORAL_TLS_SERVER_NAME")
	}
	return result, nil
}

func generateCACertPool() (*x509.CertPool, error) {
	caCert, err := os.ReadFile(os.Getenv("TEMPORAL_TLS_CA"))
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("couldn't add ca cert")
	}
	log.Printf("Loaded CA cert: %s", os.Getenv("TEMPORAL_TLS_CA"))
	return caCertPool, nil
}

func generateTLSCert() (tls.Certificate, error) {
	cert := os.Getenv("TEMPORAL_TLS_CERT")
	key := os.Getenv("TEMPORAL_TLS_KEY")
	tlsCert, err := tls.LoadX509KeyPair(cert, key)
	if err == nil {
		log.Printf("Loaded TLS key & cert: %s and %s", os.Getenv("TEMPORAL_TLS_KEY"), os.Getenv("TEMPORAL_TLS_CERT"))
	}
	return tlsCert, err
}
