package server

import (
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"net/url"
	"regexp"
	"strconv"
)

type Error string

func (e Error) Error() string {
	return string(e)
}

var pattern, _ = regexp.Compile("^(?P<collection>[A-z_]+).(?P<field>[A-z_]+)$")

func (p InheritPattern) VerifyDB(db *mongo.Database) bool {
	for _, param := range p.Params {
		match := pattern.FindStringSubmatch(param)
		c := match[1]
		//f := match[2]
		hasCollections(db, c)
	}
	// ***
	return true
}

func makestrset(schan *chan string, guide int) []string {
	m := make(map[string]int)
	ans := make([]string, 0, guide)
	for {
		str, ok := <-*schan
		if !ok {
			return ans
		}
		if _, ok := m[str]; !ok {
			m[str] = 1
			ans = append(ans, str)
		}
	}
}

func strset(slist []string) []string {
	guide := len(slist)
	m := make(map[string]int)
	ans := make([]string, 0, guide)
	for _, str := range slist {
		if _, ok := m[str]; !ok {
			m[str] = 1
			ans = append(ans, str)
		}
	}
	return ans
}

func SplitParam(param string) (collection string, field string) {
	res := pattern.FindStringSubmatch(param)
	return res[1], res[2]
}

func (p InheritPattern) RelatedCollections() []string {
	ans := make(chan []string, 1)
	schan := make(chan string)
	go func() { ans <- makestrset(&schan, len(p.Params)) }()
	for _, param := range p.Params {
		schan <- pattern.FindStringSubmatch(param)[1]
	}
	close(schan)
	return <-ans
}

func (p InheritPattern) Valid() bool {
	for _, param := range p.Params {
		if !pattern.MatchString(param) {
			return false
		}
	}
	return true
}

func (f Field) GetField() *FieldType {
	return GetField(f.FieldType)
}

func (f Field) Valid() bool {
	//if f.GetField() == nil {
	//	return false
	//}
	for _, pattern := range f.InheritPatterns {
		if !pattern.Valid() {
			return false
		}
	}
	return true
}

func (f Field) RelatedCollections() []string {
	ans := make(chan []string, 1)
	collectionNames := make(chan string)
	go func() { ans <- makestrset(&collectionNames, len(f.InheritPatterns)) }()
	for _, p := range f.InheritPatterns {
		for _, param := range p.Params {
			collectionNames <- pattern.FindStringSubmatch(param)[1]
		}
	}
	close(collectionNames)
	return <-ans
}

func (t TableForm) RelatedCollections() []string {
	ans := make(chan []string, 1)
	collectionNames := make(chan string)
	for _, f := range t.Fields {
		for _, p := range f.InheritPatterns {
			for _, param := range p.Params {
				collectionNames <- pattern.FindStringSubmatch(param)[1]
			}
		}
	}
	close(collectionNames)
	return <-ans
}

func (t TableForm) Valid() bool {
	for _, f := range t.Fields {
		if !f.Valid() {
			return false
		}
	}
	return true
}

type InheritPattern struct {
	Format string   `form:"format"`
	Params []string `form:"params"`
}

type Field struct {
	Name            string           `form:"name"`
	FieldType       string           `form:"fieldType"`
	InheritPatterns []InheritPattern `form:"inheritPatterns"`

	//DisplayName string
	//Description string
}

type TableForm struct {
	Name   string  `form:"name"`
	Fields []Field `form:"fields"`
}

type FieldType struct {
	Name   string
	Parse  func(string) (interface{}, error)
	String func(interface{}) (string, error)
}

func GetField(field string) *FieldType {
	switch field {
	case "IntField":
		return &IntField
	case "URLField":
		return &URLField
	default:
		return nil
	}
}

var IntField = FieldType{
	"IntField",
	func(value string) (interface{}, error) {
		v, err := strconv.ParseInt(string(value), 10, 0)
		if err != nil {
			return 0, err
		}
		return int(v), nil
	},
	func(v interface{}) (string, error) {
		return fmt.Sprint(v), nil
	},
}

var URLField = FieldType{
	"URLField",
	func(value string) (interface{}, error) {
		return url.Parse(value)
	},
	func(v interface{}) (string, error) {
		switch vv := v.(type) {
		case *url.URL:
			return vv.String(), nil
		default:
			return "", Error("Bad type")
		}
	},
}

//type FieldType interface {
//	Parse() (interface{}, error)
//}
//
//type IntField struct {
//	Name string
//	Value string
//	Description string
//}
//
//func (f IntField) Parse() (interface{}, error) {
//	i, err := strconv.ParseInt(f.Value, 10, 0)
//	return int(i), err
//}
