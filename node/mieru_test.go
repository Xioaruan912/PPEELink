package node

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
)

const testMierusLink = "mierus://primary:example-secret@192.0.2.1?handshake-mode=HANDSHAKE_NO_WAIT&mtu=1400&multiplexing=MULTIPLEXING_OFF&port=6901&profile=default&protocol=TCP"

func TestMierusNodeRecognition(t *testing.T) {
	p, err := ParseOutbound(testMierusLink)
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "mieru" || p.Name != "192.0.2.1:6901" || p.Server != "192.0.2.1" || p.Port != 6901 {
		t.Fatalf("unexpected identity: type=%s name=%s host=%s port=%d", p.Type, p.Name, p.Server, p.Port)
	}
	if host, port := ExtractServerHost(testMierusLink); host != "192.0.2.1" || port != 6901 {
		t.Fatalf("query port not recognized: %s:%d", host, port)
	}
}

func TestMierusRenamePreservesCredentialsAndParameters(t *testing.T) {
	link := "mierus://user%40name:p%3Aa%26ss@[2001:db8::1]?port=6901-6903&port=7901&protocol=TCP&protocol=UDP&profile=default&mtu=1400&multiplexing=MULTIPLEXING_OFF&handshake-mode=HANDSHAKE_NO_WAIT&future-option=keep#old"
	p, err := ParseOutbound(link)
	if err != nil {
		t.Fatal(err)
	}
	if p.Server != "2001:db8::1" || p.Port != 6901 || p.Name != "old" {
		t.Fatal("IPv6, port range or fragment was not recognized")
	}
	u, err := url.Parse(p.WithName("新节点 JP").Link)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	if u.Fragment != "新节点 JP" || u.User.Username() != "user@name" || password != "p:a&ss" || u.Hostname() != "2001:db8::1" {
		t.Fatal("rename changed connection identity")
	}
	original, _ := url.Parse(link)
	if !reflect.DeepEqual(u.Query(), original.Query()) {
		t.Fatal("rename lost or reordered port/protocol bindings or other parameters")
	}
	p.Server, p.Port = "mieru.example.com", 8901
	u, err = url.Parse(p.ToLink())
	if err != nil {
		t.Fatal(err)
	}
	if u.Hostname() != "mieru.example.com" || !reflect.DeepEqual(u.Query()["port"], []string{"8901", "7901"}) {
		t.Fatal("endpoint editing failed to update the query port")
	}
	p.Port = 6901
	restored, _ := url.Parse(p.ToLink())
	if restored.Query().Get("port") != "6901-6903" {
		t.Fatal("encoding mutated the original parameters")
	}
}

func TestMierusRejectsInvalidConnectionWithoutLeakingCredentials(t *testing.T) {
	for _, query := range []string{
		"profile=default&protocol=TCP",
		"profile=default&port=0&protocol=TCP",
		"profile=default&port=65536&protocol=TCP",
		"profile=default&port=6903-6901&protocol=TCP",
		"profile=default&port=6901-6903-6905&protocol=TCP",
		"profile=default&port=6901",
		"profile=default&port=6901&protocol=QUIC",
		"profile=default&port=6901&port=7901&protocol=TCP",
		"port=6901&protocol=TCP",
		"profile=a&profile=b&port=6901&protocol=TCP",
		"profile=default&port=6901&protocol=TCP&bad=%zz",
	} {
		_, err := ParseOutbound("mierus://primary:example-secret@192.0.2.1?" + query)
		if err == nil {
			t.Errorf("accepted invalid query: %s", query)
		} else if strings.Contains(err.Error(), "example-secret") {
			t.Fatal("validation error exposed credentials")
		}
	}
	for _, authority := range []string{"192.0.2.1", "primary@192.0.2.1", "primary:@192.0.2.1", "primary:example-secret@"} {
		if _, err := ParseOutbound("mierus://" + authority + "?profile=default&port=6901&protocol=TCP"); err == nil {
			t.Fatal("accepted missing host or credentials")
		}
	}
}
