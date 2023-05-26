package xmlrpc

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Array []interface{}
type Struct map[string]interface{}

func next(p *xml.Decoder) (xml.Name, interface{}, error) {
	se, nextErr := nextStart(p)
	if nextErr != nil {
		return xml.Name{}, nil, nextErr
	}
    return nextElmt(p, se)
}

func nextElmt(p *xml.Decoder, se *xml.StartElement) (xml.Name, interface{}, error) {

    var nv interface{}

	switch se.Name.Local {

	case "string":
		var s string
		if e := p.DecodeElement(&s, se); e != nil {
			return xml.Name{}, nil, e
		}
		return xml.Name{}, s, nil

	case "boolean":
		var s string
		if e := p.DecodeElement(&s, se); e != nil {
			return xml.Name{}, nil, e
		}
		s = strings.TrimSpace(s)
		var b bool
		switch s {
		case "true", "1":
			b = true
		case "false", "0":
			b = false
		default:
			return xml.Name{}, b, errors.New("invalid boolean value")
		}
		return xml.Name{}, b, nil

	case "int", "i1", "i2", "i4", "i8":
		var s string
		var i int
		if e := p.DecodeElement(&s, se); e != nil {
			return xml.Name{}, nil, e
		}
		i, e := strconv.Atoi(strings.TrimSpace(s))
		return xml.Name{}, i, e

	case "double":
		var s string
		var f float64
		if e := p.DecodeElement(&s, se); e != nil {
			return xml.Name{}, nil, e
		}
		f, e := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return xml.Name{}, f, e

	case "dateTime.iso8601":
		var s string
		if e := p.DecodeElement(&s, se); e != nil {
			return xml.Name{}, nil, e
		}
		t, e := time.Parse("20060102T15:04:05", s)
		if e != nil {
			t, e = time.Parse("2006-01-02T15:04:05-07:00", s)
			if e != nil {
				t, e = time.Parse("2006-01-02T15:04:05", s)
			}
		}
		return xml.Name{}, t, e

   	case "base64":
		var s string
		if e := p.DecodeElement(&s, se); e != nil {
			return xml.Name{}, nil, e
		}
		if b, e := base64.StdEncoding.DecodeString(s); e != nil {
			return xml.Name{}, nil, e
		} else {
			return xml.Name{}, b, nil
		}

	case "nil":
		return xml.Name{}, nil, nil

	case "value":
        return nextValue(p)

	case "struct":
        return nextStruct(p)

	case "array":
        return nextArray(p)

	case "param":
	 	return nextValue(p)

	case "params":
        return nextParams(p)

	case "fault":
		_, value, _ := next(p)
		fs, ok := value.(Struct)
		if !ok {
			return xml.Name{}, value, fmt.Errorf("fault: wanted Struct, got %#v", value)
		}
		var f Fault
        switch code := fs["faultCode"].(type) {
        case string:
            f.Code, _ = strconv.Atoi(code)
        case int:
            f.Code = code
        }
		f.Message, _ = fs["faultString"].(string)
		return xml.Name{}, nil, &f
	}

	if e := p.DecodeElement(&nv, se); e != nil {
		return xml.Name{}, nil, e
	}
	return se.Name, nv, nil
}

func nextStart(p *xml.Decoder) (*xml.StartElement, error) {
	for {
		t, e := p.Token()
		if e != nil {
			return &xml.StartElement{}, e
		}
		switch t := t.(type) {
		case xml.StartElement:
			return &t, nil
		}
	}
}

func nextStruct(p *xml.Decoder) (xml.Name, interface{}, error) {

    const (
        structStart = iota
        structMember
        structValue
    )

    var (
        name string
        st   Struct = make(Struct)
    )
    
    state := structStart
    for {
        t, e := p.Token()
		if e != nil {
			return xml.Name{}, nil, e
		}

        switch t := t.(type) {
		case xml.StartElement:
            switch state {
            case structStart:
                if t.Name.Local != "member" {
                    return xml.Name{}, nil, errors.New("expected member")
                }
                state = structMember
            case structMember:
                if t.Name.Local != "name" {
                    return xml.Name{}, nil, errors.New("expected name")
                }
                if e := p.DecodeElement(&name, &t); e != nil {
                    return xml.Name{}, nil, e
                }
                state = structValue
            case structValue:
                if t.Name.Local != "value" {
                    return xml.Name{}, nil, errors.New("expected value")
                }
                _, v, e := nextValue(p)
                if e != nil {
                    return xml.Name{}, nil, e
                }
                st[name] = v
                state = structStart
            }
        case xml.EndElement:
            if t.Name.Local == "struct" {
                switch state {
                case structMember, structValue:
                    return xml.Name{}, nil, errors.New("unexpected end of struct")
                }
                return xml.Name{}, st, nil
            }
        }
    }
}

func nextArray(p *xml.Decoder) (xml.Name, interface{}, error) {

    const (
        arrayStart = iota
        arrayData
    )

    var ar Array = make(Array,0)
    
    state := arrayStart
    for {
        t, e := p.Token()
		if e != nil {
			return xml.Name{}, nil, e
		}

        switch t := t.(type) {
		case xml.StartElement:
            switch state {
            case arrayStart:
                if t.Name.Local != "data" {
                    return xml.Name{}, nil, errors.New("expected data")
                }
                state = arrayData
            case arrayData:
                if t.Name.Local != "value" {
                    return xml.Name{}, nil, errors.New("expected value")
                }
                _, v, e := nextValue(p)
                if e != nil {
                    return xml.Name{}, nil, e
                }
                ar = append(ar, v)
            }
        case xml.EndElement:
            if t.Name.Local == "data" {
                return xml.Name{}, ar, nil
            }
        }
    }
}

func nextParams(p *xml.Decoder) (xml.Name, interface{}, error) {

    var ar Array = make(Array,0)
    for {
        t, e := p.Token()
		if e != nil {
			return xml.Name{}, nil, e
		}

        switch t := t.(type) {
		case xml.StartElement:
            if t.Name.Local != "param" {
                return xml.Name{}, nil, errors.New("expected param")
            }
            _, v, e := nextValue(p)
            if e != nil {
                return xml.Name{}, nil, e
            }
            ar = append(ar, v)

        case xml.EndElement:
            if t.Name.Local == "params" {
                return xml.Name{}, ar, nil
            }
        }
    }
}

func nextValue(p *xml.Decoder) (xml.Name, interface{}, error) {

    var (
        str   string
        obj   interface{}
    )

	for {
		t, e := p.Token()
		if e != nil {
			return xml.Name{}, nil, e
		}

		switch t := t.(type) {

		case xml.StartElement:
            _, v, e := nextElmt(p,&t)
            if e != nil {
                return xml.Name{}, nil, e
            }
            obj = v

        case xml.CharData:
            str += string(t)

		case xml.EndElement:
            if obj == nil {
                //fmt.Printf("EndElement: %s (str=%+v)\n", t.Name.Local, str)
                return xml.Name{}, str, nil
            } else {
                //fmt.Printf("EndElement: %s (obj=%+v)\n", t.Name.Local, obj)
                return xml.Name{}, obj, nil
            }
		}
	}
}

var UnsupportedType = errors.New("unsupported type")

func writeXML(w io.Writer, v interface{}, typ bool) error {
	if v == nil {
		_, err := io.WriteString(w, "<nil/>")
		return err
	}
	r := reflect.ValueOf(v)
	t := r.Type()
	k := t.Kind()

	if b, ok := v.([]byte); ok {
		io.WriteString(w, "<base64>")
		_, err := base64.NewEncoder(base64.StdEncoding, w).Write(b)
		io.WriteString(w, "</base64>")
		return err
	}

	switch k {
	case reflect.Invalid:
		return UnsupportedType
	case reflect.Bool:
		_, err := fmt.Fprintf(w, "<boolean>%v</boolean>", v)
		return err
	case reflect.Int,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if typ {
			_, err := fmt.Fprintf(w, "<int>%v</int>", v)
			return err
		}
		_, err := fmt.Fprintf(w, "%v", v)
		return err
	case reflect.Uintptr:
		return UnsupportedType
	case reflect.Float32, reflect.Float64:
		if typ {
			_, err := fmt.Fprintf(w, "<double>%v</double>", v)
			return err
		}
		_, err := fmt.Fprintf(w, "%v", v)
		return err
	case reflect.Complex64, reflect.Complex128:
		return UnsupportedType
	case reflect.Array:
		io.WriteString(w, "<array><data>")
		for n := 0; n < r.Len(); n++ {
			io.WriteString(w, "<value>")
			err := writeXML(w, r.Index(n).Interface(), typ)
			io.WriteString(w, "</value>")
			if err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "</data></array>")
		return err
	case reflect.Chan:
		return UnsupportedType
	case reflect.Func:
		return UnsupportedType
	case reflect.Interface:
		return writeXML(w, r.Elem(), typ)
	case reflect.Map:
		io.WriteString(w, "<struct>")
		for _, key := range r.MapKeys() {
			io.WriteString(w, "<member><name>")
			if err := xml.EscapeText(w, []byte(key.Interface().(string))); err != nil {
				return err
			}
			io.WriteString(w, "</name><value>")
			if err := writeXML(w, r.MapIndex(key).Interface(), typ); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "</value></member>"); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "</struct>")
		return err
	case reflect.Ptr:
		return UnsupportedType
	case reflect.Slice:
		io.WriteString(w, "<array><data>")
		for n := 0; n < r.Len(); n++ {
			io.WriteString(w,"<value>")
			writeXML(w, r.Index(n).Interface(), typ)
			io.WriteString(w, "</value>")
		}
		_, err := io.WriteString(w, "</data></array>")
		return err
	case reflect.String:
		if typ {
			io.WriteString(w, "<string>")
		}
		err := xml.EscapeText(w, []byte(v.(string)))
		if typ {
			io.WriteString(w, "</string>")
		}
		return err
	case reflect.Struct:
		io.WriteString(w, "<struct>")
		for n := 0; n < r.NumField(); n++ {
			fmt.Fprintf(w, "<member><name>%s</name><value>", t.Field(n).Name)
			if err := writeXML(w, r.FieldByIndex([]int{n}).Interface(), true); err != nil {
				return err
			}
			io.WriteString(w, "</value></member>")
		}
		_, err := io.WriteString(w, "</struct>")
		return err
	case reflect.UnsafePointer:
		return writeXML(w, r.Elem(), typ)
	}
	return nil
}

// Client is client of XMLRPC
type Client struct {
	HttpClient *http.Client
	url        string
}

// NewClient create new Client
func NewClient(url string) *Client {
	return &Client{
		HttpClient: &http.Client{Transport: http.DefaultTransport, Timeout: 10 * time.Second},
		url:        url,
	}
}

func Marshal(w io.Writer, name string, args ...interface{}) error {
	io.WriteString(w, `<?xml version="1.0"?>`)
	var end string
	if name == "" {
		io.WriteString(w, "<methodResponse>")
		end = "</methodResponse>"
	} else {
		io.WriteString(w, "<methodCall><methodName>")
		if err := xml.EscapeText(w, []byte(name)); err != nil {
			return err
		}
		io.WriteString(w, "</methodName>")
		end = "</methodCall>"
	}

	io.WriteString(w, "<params>")
	for _, arg := range args {
		io.WriteString(w, "<param><value>")
		if err := writeXML(w, arg, true); err != nil {
			return err
		}
		io.WriteString(w, "</value></param>")
	}
	io.WriteString(w, "</params>")
	_, err := io.WriteString(w, end)
	return err
}

func makeRequest(name string, args ...interface{}) *bytes.Buffer {
	var buf bytes.Buffer
	if err := Marshal(&buf, name, args...); err != nil {
		panic(err)
	}
	return &buf
}

func call(client *http.Client, url, name string, args ...interface{}) (v Array, e error) {
	r, e := client.Post(url, "text/xml", makeRequest(name, args...))
	if e != nil {
		return nil, e
	}

	// Since we do not always read the entire body, discard the rest, which
	// allows the http transport to reuse the connection.
	defer io.Copy(ioutil.Discard, r.Body)
	defer r.Body.Close()

	if r.StatusCode/100 != 2 {
		return nil, errors.New(http.StatusText(http.StatusBadRequest))
	}

	_, v, e = Unmarshal(r.Body)
	return v, e
}

func Unmarshal(r io.Reader) (string, Array, error) {
	var name string
	p := xml.NewDecoder(r)
	se, e := nextStart(p) // methodResponse
	if e != nil {
		return name, nil, e
	}
	if se.Name.Local != "methodResponse" {
		if se.Name.Local != "methodCall" {
			return name, nil, errors.New("invalid response: missing methodResponse")
		}
		if se, e = nextStart(p); e != nil {
			return name, nil, e
		}
		if se.Name.Local != "methodName" {
			return name, nil, errors.New("invalid response: missing methodName")
		}
		if e = p.DecodeElement(&name, se); e != nil {
			return name, nil, e
		}
	}
	_, v, e := next(p)
	if a, ok := v.(Array); ok {
		return name, a, e
	} else if e == nil {
		e = fmt.Errorf("wanted Array, got %#v", v)
	}
	return name, nil, e
}

type Fault struct {
	Code    int
	Message string
}

func (f *Fault) Error() string { return fmt.Sprintf("%d: %s", f.Code, f.Message) }

// Call call remote procedures function name with args
func (c *Client) Call(name string, args ...interface{}) (v Array, e error) {
	return call(c.HttpClient, c.url, name, args...)
}

// Call call remote procedures function name with args
func Call(url, name string, args ...interface{}) (v Array, e error) {
	return call(http.DefaultClient, url, name, args...)
}
