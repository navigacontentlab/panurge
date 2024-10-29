package cockroach

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	_ "github.com/lib/pq" //nolint:nolintlint
)

const (
	ssmPrefix = "/cockroach/certs/clients"
)

// DefaultConnection sets up a database connection to the provided
// host using the application name as both username and database name.
func DefaultConnection(ctx context.Context, host, application string) (*sql.DB, error) {
	cc, err := NewConnectionConfig(
		ctx,
		application,
		ConnectionOptions{
			Host: host,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to set up database connection configuration: %w", err)
	}

	return Connect(ctx, cc, application)
}

// ConnectionOptions are used to control how we connect to the
// cluster.
type ConnectionOptions struct {
	SSM                  *ssm.SSM
	CertificateDirectory string
	DatabaseParameters   url.Values
	Host                 string
}

// ConnectionConfig is a database configuration that can be used to
// create Cockroach database connection URLs.
type ConnectionConfig struct {
	certDir     string
	user        string
	host        string
	dbParams    url.Values
	credentials *Credentials
}

// Credentials are the credentials used to connect to and verify the
// identity of the database cluster.
type Credentials struct {
	CA          string `json:"ca"`
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
}

// NewConnectionconfig creates a new configuration for a given user
// and host.
func NewConnectionConfig(
	ctx context.Context,
	user string,
	opts ConnectionOptions,
) (*ConnectionConfig, error) {
	if opts.Host == "" {
		return nil, errors.New("missing database host")
	}

	ssmSvc := opts.SSM
	if ssmSvc == nil {
		sess, err := session.NewSession()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to set up AWS SDK session: %w", err)
		}

		ssmSvc = ssm.New(sess)
	}

	cred, err := fetch(ctx, ssmSvc, ssmPrefix, user)
	if err != nil {
		return nil, err
	}

	certDir := opts.CertificateDirectory
	if certDir == "" {
		certDir, err = os.MkdirTemp("", user)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to create temporary certificate directory: %w", err)
		}
	}

	cc := ConnectionConfig{
		certDir:     certDir,
		host:        opts.Host,
		user:        user,
		credentials: cred,
		dbParams:    opts.DatabaseParameters,
	}

	if err := cc.createCertDirectory(); err != nil {
		return nil, err
	}

	return &cc, nil
}

// DatabaseURL creates a database URL for use with sql.Open.
func (cc *ConnectionConfig) DatabaseURL(database string) string {
	dbValues := make(url.Values)

	dbValues.Set("connect_timeout", "5")

	for k, v := range cc.dbParams {
		dbValues[k] = v
	}

	dbValues.Set("sslmode", "verify-full")
	dbValues.Set("sslcert", filepath.Join(
		cc.certDir, "client."+cc.user+".crt",
	))
	dbValues.Set("sslkey", filepath.Join(
		cc.certDir, "client."+cc.user+".key",
	))
	dbValues.Set("sslrootcert", filepath.Join(
		cc.certDir, "ca.crt",
	))

	dbURL := &url.URL{
		Scheme:   "postgresql",
		User:     url.User(cc.user),
		Host:     cc.host,
		Path:     database,
		RawQuery: dbValues.Encode(),
	}

	return dbURL.String()
}

// CertificateDir returns the directory used for storing certificates.
func (cc *ConnectionConfig) CertificateDir() string {
	return cc.certDir
}

func fetch(
	ctx context.Context,
	ssmSvc *ssm.SSM, prefix string, name string,
) (*Credentials, error) {
	paramName := filepath.Join(prefix, name)
	res, err := ssmSvc.GetParameterWithContext(ctx, &ssm.GetParameterInput{
		Name:           &paramName,
		WithDecryption: aws.Bool(true),
	})

	if err != nil {
		return nil, fmt.Errorf(
			"failed to fetch certificate: %w", err)
	}

	var response Credentials

	value := []byte(*res.Parameter.Value)

	if err := json.Unmarshal(value, &response); err != nil {
		return nil, fmt.Errorf(
			"failed to parse stored credentials: %w", err)
	}

	return &response, nil
}

func (cc *ConnectionConfig) createCertDirectory() error {
	if err := os.MkdirAll(cc.certDir, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	files := map[string][]byte{
		"ca.crt":                     []byte(cc.credentials.CA),
		"client." + cc.user + ".crt": []byte(cc.credentials.Certificate),
		"client." + cc.user + ".key": []byte(cc.credentials.Key),
	}
	for name, data := range files {
		err := os.WriteFile(
			filepath.Join(cc.certDir, name), data, 0600,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to create file %q: %w",
				name, err)
		}
	}

	return nil
}

// Connect to the cluster using a connection configuration.
func Connect(
	ctx aws.Context,
	cc *ConnectionConfig, database string,
) (*sql.DB, error) {
	db, err := sql.Open("postgres", cc.DatabaseURL(database))
	if err != nil {
		return nil, fmt.Errorf(
			"failed to configure database connection: %w",
			err,
		)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf(
			"failed to connect to database: %w", err)
	}

	return db, nil
}
