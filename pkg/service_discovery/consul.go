package service_discovery

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"net/url"
	"strconv"
	"strings"
)

type Consul struct {
	address  string
	schema   string
	certFile string
	keyFile  string

	username string
	password string
	token    string
}

type ConsulOption func(c *Consul)

func NewConsul(options ...ConsulOption) *Consul {
	c := &Consul{}
	for _, option := range options {
		option(c)
	}
	return c
}

func WithAddress(address string) ConsulOption {
	return func(c *Consul) {
		c.address = address
	}
}

func WithSchema(schema string) ConsulOption {
	return func(c *Consul) {
		c.schema = schema
	}
}

func WithBasicAuth(username, password string) ConsulOption {
	return func(c *Consul) {
		c.username = username
		c.password = password
	}
}
func WithTLSFiles(certFile, keyFile string) ConsulOption {
	return func(c *Consul) {
		c.certFile = certFile
		c.keyFile = keyFile
	}
}
func WithToken(token string) ConsulOption {
	return func(c *Consul) {
		c.token = token
	}
}

func (c *Consul) Default() {
	if c.address == "" {
		c.address = "127.0.0.1:8500"
	}
	if c.schema == "" {
		c.schema = "http"
	}
}

func (c *Consul) Watch(service ServiceInfo) error {
	c.Default()
	config := api.DefaultConfig()
	config.Address = c.address
	config.Scheme = c.schema
	config.TLSConfig.CertFile = c.certFile
	config.TLSConfig.KeyFile = c.keyFile
	if c.username != "" {
		config.HttpAuth = &api.HttpBasicAuth{
			Username: c.username,
			Password: c.password,
		}
	}
	config.Token = c.token
	client, err := api.NewClient(config)
	if err != nil {
		return err
	}

	service.Default()
	registration := new(api.AgentServiceRegistration)
	registration.Name = service.Name
	addr, err := url.Parse(service.Address)
	if err != nil {
		return err
	}
	hostWithPort := strings.Split(addr.Host, ":")
	registration.Address = hostWithPort[0]
	if len(hostWithPort) == 2 {
		port, _ := strconv.ParseInt(hostWithPort[1], 10, 64)
		registration.Port = int(port)
	}
	registration.Tags = service.Tags
	registration.Check = &api.AgentServiceCheck{
		HTTP:                           fmt.Sprintf("%s%s", service.Address, service.HealthPath),
		TLSSkipVerify:                  true,
		Timeout:                        fmt.Sprintf("%ds", service.Timeout),
		Interval:                       fmt.Sprintf("%ds", service.Interval),
		DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", service.DeregisterTime),
	}

	if err = client.Agent().ServiceRegister(registration); err != nil {
		return err
	}

	return nil
}
