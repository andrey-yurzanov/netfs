package api

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var ErrLocalIPNotFound = errors.New("local IP address not found")
var rfc1918 = []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}

const ContentType = "Content-Type"
const JsonContentType = "application/json"
const BinaryContentType = "application/octet-stream"

const cidrEnd = "1.0/24"
const ipSeparator = "."
const decimalBase = 10

type NetworkConfig struct {
	Port    int
	Timeout time.Duration
}

type Network struct {
	Config NetworkConfig
	client *http.Client
}

func (network *Network) Hosts() ([]Host, error) {
	var hosts []Host

	ips, err := network.ips()
	if err == nil {
		callback := make(chan *Host)
		for _, ip := range ips {
			go func(ip net.IP, callback chan *Host) {
				host, _ := network.Host(ip)
				callback <- host
			}(ip, callback)
		}

		for range ips {
			if host := <-callback; host != nil {
				hosts = append(hosts, *host)
			}
		}
	}
	return hosts, err
}

func (network *Network) Host(ip net.IP) (*Host, error) {
	res, err := network.client.Get(BuildUrl(ip, network.Config.Port, "/api/host"))
	if err == nil {
		defer res.Body.Close()

		return Unmarshal(res.Body, &Host{Network: network})
	}
	return nil, err
}

func (network *Network) LocalHost() (*Host, error) {
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		var ips []net.IP
		for _, a := range addrs {
			if val, ok := a.(*net.IPAddr); ok {
				ips = append(ips, val.IP)
			} else if val, ok := a.(*net.IPNet); ok {
				ips = append(ips, val.IP)
			}
		}

		var localIP net.IP
		for _, cidr := range rfc1918 {
			_, block, _ := net.ParseCIDR(cidr)
			for _, ip := range ips {
				if block.Contains(ip) {
					localIP = ip
					break
				}
			}
		}

		if localIP != nil {
			var hostname string
			if hostname, err = os.Hostname(); err == nil {
				return &Host{Name: hostname, IP: localIP, Network: network}, nil
			}
		} else {
			err = ErrLocalIPNotFound
		}
	}
	return nil, err
}

func (network *Network) ips() ([]net.IP, error) {
	ips := []net.IP{}
	local, err := network.LocalHost()
	if err == nil {
		localString := local.IP.String()
		parts := strings.Split(localString, ipSeparator)
		cidr := strings.Join([]string{parts[0], parts[1], cidrEnd}, ipSeparator)

		var prefix netip.Prefix
		if prefix, err = netip.ParsePrefix(cidr); err == nil {
			prefix = prefix.Masked()
			addr := prefix.Addr()
			for prefix.Contains(addr) {
				ip := addr.String()
				if localString != "" {
					ips = append(ips, net.ParseIP(ip))
				}
				addr = addr.Next()
			}
		}
	}
	return ips, err
}

func NewNetwork(config NetworkConfig) *Network {
	return &Network{Config: config, client: &http.Client{Timeout: config.Timeout}}
}

func Unmarshal[T any](reader io.Reader, value T) (T, error) {
	data, err := io.ReadAll(reader)
	if err == nil {
		err = json.Unmarshal(data, value)
	}
	return value, err
}

func UnmarshalArray[T any](reader io.Reader, value *[]T) ([]T, error) {
	data, err := io.ReadAll(reader)
	if err == nil {
		if err = json.Unmarshal(data, value); err == nil {
			return *value, nil
		}
	}
	return nil, err
}

func BuildUrl(ip net.IP, port int, endpoint string, params ...string) string {
	newUrl := url.URL{
		Scheme: "http",
		Host:   ip.String() + ":" + strconv.Itoa(port),
		Path:   endpoint,
	}

	if len(params) > 0 {
		queryParams := url.Values{}

		index := 0
		for index < len(params)-1 {
			queryParams.Add(params[index], params[index+1])
			index++
		}
		newUrl.RawQuery = queryParams.Encode()
	}
	return newUrl.String()
}
