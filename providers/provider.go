package providers

// Provider is the interface that wraps the basic FetchIPs method.
//
// FetchIPs should return a slice of IP strings and an error if any occurred.
type Provider interface {
	FetchIPs() ([]string, error)
}
