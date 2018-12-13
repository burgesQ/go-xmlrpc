package xmlrpc

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
    "bytes"
    "reflect"
)

func createServer(path, name string, f func(args ...interface{}) (interface{}, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		p := xml.NewDecoder(r.Body)
		se, _ := nextStart(p) // methodResponse
		if se.Name.Local != "methodCall" {
			http.Error(w, "missing methodCall", http.StatusBadRequest)
			return
		}
		se, _ = nextStart(p) // params
		if se.Name.Local != "methodName" {
			http.Error(w, "missing methodName", http.StatusBadRequest)
			return
		}
		var s string
		if err := p.DecodeElement(&s, se); err != nil {
			http.Error(w, "wrong function name", http.StatusBadRequest)
			return
		}
		if s != name {
			http.Error(w, fmt.Sprintf("want function name %q but got %q", name, s), http.StatusBadRequest)
			return
		}
		se, _ = nextStart(p) // params
		if se.Name.Local != "params" {
			http.Error(w, "missing params", http.StatusBadRequest)
			return
		}
		var args []interface{}
		for {
			se, _ = nextStart(p) // param
			if se.Name.Local == "" {
				break
			}
			if se.Name.Local != "param" {
				http.Error(w, "missing param", http.StatusBadRequest)
				return
			}
			se, _ = nextStart(p) // value
			if se.Name.Local != "value" {
				http.Error(w, "missing value", http.StatusBadRequest)
				return
			}
			_, v, err := next(p)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			args = append(args, v)
		}

		ret, err := f(args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Write([]byte(`
		<?xml version="1.0"?>
		<methodResponse>
		<params>
			<param>
				<value>` + toXml(ret, true) + `</value>
			</param>
		</params>
		</methodResponse>
		`))
	}
}

func TestAddInt(t *testing.T) {
	ts := httptest.NewServer(createServer("/api", "AddInt", func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, errors.New("bad number of arguments")
		}
		switch args[0].(type) {
		case int:
		default:
			return nil, errors.New("args[0] should be int")
		}
		switch args[1].(type) {
		case int:
		default:
			return nil, errors.New("args[1] should be int")
		}
		return args[0].(int) + args[1].(int), nil
	}))
	defer ts.Close()

	client := NewClient(ts.URL + "/api")
	v, err := client.Call("AddInt", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	i, ok := v[0].(int)
	if !ok {
		t.Fatalf("want int but got %#v", v)
	}
	if i != 3 {
		t.Fatalf("want %v but got %#v", 3, v)
	}
}

func TestAddString(t *testing.T) {
	ts := httptest.NewServer(createServer("/api", "AddString", func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, errors.New("bad number of arguments")
		}
		switch args[0].(type) {
		case string:
		default:
			return nil, errors.New("args[0] should be string")
		}
		switch args[1].(type) {
		case string:
		default:
			return nil, errors.New("args[1] should be string")
		}
		return args[0].(string) + args[1].(string), nil
	}))
	defer ts.Close()

	client := NewClient(ts.URL + "/api")
	v, err := client.Call("AddString", "hello", "world")
	if err != nil {
		t.Fatal(err)
	}
	s, ok := v[0].(string)
	if !ok {
		t.Fatalf("want string but got %#v", v)
	}
	if s != "helloworld" {
		t.Fatalf("want %q but got %q", "helloworld", v)
	}
}

type ParseStructArrayHandler struct {
}

func (h *ParseStructArrayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`
<?xml version="1.0"?>
<methodResponse>
  <params>
    <param>
      <value>
        <array>
          <data>
            <value>
              <struct>
                <member>
                  <name>test1</name>
                  <value>
                    <string>a</string>
                  </value>
                </member>
                <member>
                  <name>test2</name>
                  <value>
                    <int>2</int>
                  </value>
                </member>
              </struct>
            </value>
            <value>
              <struct>
                <member>
                  <name>test1</name>
                  <value>
                    <string>b</string>
                  </value>
                </member>
                <member>
                  <name>test2</name>
                  <value>
                    <int>2</int>
                  </value>
                </member>
              </struct>
            </value>
            <value>
              <struct>
                <member>
                  <name>test1</name>
                  <value>
                    <string>c</string>
                  </value>
                </member>
                <member>
                  <name>test2</name>
                  <value>
                    <int>2</int>
                  </value>
                </member>
              </struct>
            </value>
          </data>
        </array>
      </value>
    </param>
  </params>
</methodResponse>
		`))
}

func TestParseStructArray(t *testing.T) {
	ts := httptest.NewServer(&ParseStructArrayHandler{})
	defer ts.Close()

	res, err := NewClient(ts.URL + "/").Call("Irrelevant")

	if err != nil {
		t.Fatal(err)
	}

    fmt.Printf("%+v\n",res)
    if len(res) != 1 {
		t.Fatalf("expected array with 1 entry (%+v)", res)
    }
    res = res[0].(Array)
	if len(res) != 3 {
		t.Fatalf("expected array with 3 entries (%+v)", res)
	}
}

type ParseIntArrayHandler struct {
}

func (h *ParseIntArrayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`
<?xml version="1.0"?>
<methodResponse>
  <params>
    <param>
      <value>
        <array>
          <data>
            <value>
              <int>2</int>
            </value>
            <value>
              <int>3</int>
            </value>
            <value>
              <int>4</int>
            </value>
          </data>
        </array>
      </value>
    </param>
  </params>
</methodResponse>
		`))
}

func TestParseIntArray(t *testing.T) {
	ts := httptest.NewServer(&ParseIntArrayHandler{})
	defer ts.Close()

	res, err := NewClient(ts.URL + "/").Call("Irrelevant")

	if err != nil {
		t.Fatal(err)
	}

    res = res[0].(Array)
	if len(res) != 3 {
		t.Fatal("expected array with 3 entries")
	}
}

type ParseMixedArrayHandler struct {
}

func (h *ParseMixedArrayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`
<?xml version="1.0"?>
<methodResponse>
  <params>
    <param>
      <value>
        <array>
          <data>
            <value>
              <struct>
                <member>
                  <name>test1</name>
                  <value>
                    <string>a</string>
                  </value>
                </member>
                <member>
                  <name>test2</name>
                  <value>
                    <int>2</int>
                  </value>
                </member>
              </struct>
            </value>
            <value>
              <int>2</int>
            </value>
            <value>
              <struct>
                <member>
                  <name>test1</name>
                  <value>
                    <string>b</string>
                  </value>
                </member>
                <member>
                  <name>test2</name>
                  <value>
                    <int>2</int>
                  </value>
                </member>
              </struct>
            </value>
            <value>
              <int>4</int>
            </value>
          </data>
        </array>
      </value>
    </param>
  </params>
</methodResponse>
		`))
}

func TestParseMixedArray(t *testing.T) {
	ts := httptest.NewServer(&ParseMixedArrayHandler{})
	defer ts.Close()

	res, err := NewClient(ts.URL + "/").Call("Irrelevant")

	if err != nil {
		t.Fatal(err)
	}

    res = res[0].(Array)
	if len(res) != 4 {
		t.Fatalf("expected array with 4 entries (%+v)", res)
	}
}

func TestRawStringStruct(t *testing.T) {
    payload := `
<methodResponse>
  <params>
    <param> 
      <value>
        <array>
          <data>
            <value>
              <i4>200</i4>
            </value>
            <value>OK</value>
            <value>
              <struct>
                <member>
                  <name>status</name>
                  <value>OK</value>
                </member>
                <member>
                  <name>contact</name>
                  <value>&lt;sip:raf@192.168.164.128:5060&gt;;expires=60</value>
                </member>
              </struct>
            </value>
          </data>
        </array>
      </value> 
    </param>
  </params>
</methodResponse>`

    expectedResponse := Array{
        Array{
            200, "OK",
            Struct{
                "status": "OK",
                "contact": "<sip:raf@192.168.164.128:5060>;expires=60",
            },
        },
    }
    
    _, v, e := Unmarshal(bytes.NewReader([]byte(payload)))
    if e != nil {
		t.Fatalf("could not unmarshal payload: %s", e)
    }

    if !reflect.DeepEqual(v,expectedResponse) {
         t.Fatalf("response different from expected (%+v)", v)
    }
}

func TestRawStringArray(t *testing.T) {

    payload := `<methodResponse><params><param> 
  <value><array><data>
    <value>500</value>
    <value>Call Agent does not exist</value>
  </data></array></value> 
</param></params></methodResponse>`

    expectedResponse := Array{
        Array{ "500", "Call Agent does not exist"},
    }

    _, v, e := Unmarshal(bytes.NewReader([]byte(payload)))
    if e != nil {
		t.Fatalf("could not unmarshal payload: %s", e)
    }

    if !reflect.DeepEqual(v,expectedResponse) {
        t.Fatalf("response different from expected (%+v)", v)
    }    
}

func TestRawStringNestedArray(t *testing.T) {

    payload := `<methodResponse><params><param> 
  <value><array><data>
    <value>
      <array>
        <data>
          <value>212.79.111.155</value>
          <value>  <i4>5040</i4>  </value>
          <value>  <i4>3509720</i4>  </value>
        </data>
      </array>
    </value>
    <value>
      <array>
        <data>
          <value>192.168.164.1</value>
          <value><i4>5060</i4></value>
          <value><i4>7122760</i4></value>
        </data>
      </array>
    </value>
  </data></array></value> 
</param></params></methodResponse>`

    expectedResponse := Array{
        Array{
            Array{ "212.79.111.155", 5040, 3509720 },
            Array{ "192.168.164.1", 5060, 7122760 },
        },
    }

    _, v, e := Unmarshal(bytes.NewReader([]byte(payload)))
    if e != nil {
		t.Fatalf("could not unmarshal payload: %s", e)
    }

    if !reflect.DeepEqual(v,expectedResponse) {
        t.Fatalf("response different from expected (%+v)", v)
    }    
}

func TestStruct(t *testing.T) {
    payload := `
    <struct>
      <member>
        <name>foo</name>
        <value>bar</value>
      </member>
      <member>
        <name>num</name>
        <value><i4>12345</i4></value>
      </member>
      <member>
        <name>bla</name>
        <value>blub</value>
      </member>
    </struct>`

    expectedResponse := Struct{
        "foo": "bar",
        "num": 12345,
        "bla": "blub",
    }

    p := xml.NewDecoder(bytes.NewReader([]byte(payload)))
    _, v, e := next(p)
    if e != nil {
		t.Fatalf("could not unmarshal payload: %s", e)
    }

    if !reflect.DeepEqual(v,expectedResponse) {
        t.Fatalf("response different from expected (%+v)", v)
    }    
}

func TestArray(t *testing.T) {

    payload := `
    <array>
      <data>
        <value>foo</value>
        <value><i4>1</i4></value>
        <value>bar</value>
        <value><i4>2</i4></value>
        <value>bla</value>
        <value><i4>3</i4></value>
        <value>blub</value>
      </data>
    </array>`

    expectedResponse := Array{
        "foo", 1, "bar", 2, "bla", 3, "blub",
    }

    p := xml.NewDecoder(bytes.NewReader([]byte(payload)))
    _, v, e := next(p)
    if e != nil {
		t.Fatalf("could not unmarshal payload: %s", e)
    }

    if !reflect.DeepEqual(v,expectedResponse) {
        t.Fatalf("response different from expected (%+v)", v)
    }    
}

func TestArrayOfStruct(t *testing.T) {

    payload := `
<array>
  <data>
    <value>foo</value>
    <value><i4>1</i4></value>
    <value>
    <struct>
      <member>
        <name>foo</name>
        <value>bar</value>
      </member>
      <member>
        <name>num</name>
        <value><i4>12345</i4></value>
      </member>
      <member>
        <name>bla</name>
        <value>blub</value>
      </member>
    </struct>
    </value>
    <value><i4>3</i4></value>
    <value>blub</value>
  </data>
</array>`

    expectedResponse := Array{
        "foo", 1,
        Struct{
            "foo": "bar",
            "num": 12345,
            "bla": "blub",
        },
        3, "blub",
    }

    p := xml.NewDecoder(bytes.NewReader([]byte(payload)))
    _, v, e := next(p)
    if e != nil {
		t.Fatalf("could not unmarshal payload: %s", e)
    }

    if !reflect.DeepEqual(v,expectedResponse) {
        t.Fatalf("response different from expected (%+v)", v)
    }    
}

func toXml(v interface{}, typ bool) (s string) {
	var buf strings.Builder
	if err := writeXML(&buf, v, typ); err != nil {
		panic(err)
	}
	return buf.String()
}
