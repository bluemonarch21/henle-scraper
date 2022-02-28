package server

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/websocket"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

//var db = make(map[string]string)

type MongoRepository struct {
	Database *mongo.Database
}

func (repo MongoRepository) GetAllDocuments(name string) ([]interface{}, error) {
	var products []interface{}
	var filter gin.H
	cursor, err := repo.Database.Collection(name).Find(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &products)
	if err != nil {
		return nil, err
	}
	return products, err
}

type Person struct {
	Name    string `form:"name"`
	Address string `form:"address"`
}

var db *mongo.Database

func SetDatabase(database *mongo.Database) {
	db = database
}

func EchoServer(ws *websocket.Conn) {
	io.Copy(ws, ws)
}

func SetupRouter() *gin.Engine {
	// Disable Console Color
	// gin.DisableConsoleColor()
	r := gin.Default()

	r.Use(gin.WrapH(websocket.Handler(EchoServer)))

	// Ping test
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	r.GET("/collections/list", func(c *gin.Context) {
		names, err := db.ListCollectionNames(context.TODO(), bson.M{})
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"names": names})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		}
	})

	r.POST("/collections/create", func(c *gin.Context) {
		var queryParams struct {
			Name string `form:"name"`
		}
		if err := c.ShouldBindQuery(&queryParams); err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure")
			return
		}
		if err := db.CreateCollection(context.TODO(), queryParams.Name); err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure")
			return
		}
		c.String(http.StatusOK, "Success")
	})

	r.POST("/collection/:name/insert", func(c *gin.Context) {
		name := c.Params.ByName("name")
		if !hasCollections(db, name) {
			c.String(http.StatusBadRequest, "Failure")
			return
		}
		collection := db.Collection(name)
		var items []interface{}
		err := c.ShouldBind(&items)
		if err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure")
			return
		}
		many, err := collection.InsertMany(context.TODO(), items)
		if err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure")
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "Success", "insertedIDs": many.InsertedIDs})
		return
	})

	type CreateTable struct {
		Name   string  `form:"name"`
		Fields []Field `form:"fields"`
	}

	r.POST("/tables/create", func(c *gin.Context) {
		var createTable CreateTable
		if c.ShouldBindQuery(&createTable) != nil {
			c.String(http.StatusBadRequest, "Failure")
		}
		meta := db.Collection("_meta")
		for _, field := range createTable.Fields {
			if GetField(field.FieldType) == nil {
				c.String(http.StatusBadRequest, fmt.Sprintf("FieldType %v doesn't exist", field))
			}
		}
		if _, err := meta.InsertOne(context.TODO(), createTable); err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure")
		}
		if err := db.CreateCollection(context.TODO(), createTable.Name+"_manual"); err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure")
		}
		c.String(http.StatusOK, "Success")
		return
	})

	r.POST("/tables/list", func(c *gin.Context) {
		var queryParams struct {
			Name string `form:"name"`
		}
		if err := c.ShouldBindQuery(&queryParams); err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure at query")
			return
		}

		tableForm := TableForm{}
		if err := c.ShouldBindBodyWith(&tableForm, binding.JSON); err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure at body")
			return
		}
		if !tableForm.Valid() {
			c.String(http.StatusBadRequest, "Invalid table form")
			return
		}

		if !hasCollections(db, queryParams.Name) {
			c.String(http.StatusNotFound, "Collection not found")
			return
		}

		cur, err := db.Collection(queryParams.Name).Find(context.TODO(), bson.M{})
		if err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure at find document")
			return
		}
		defer cur.Close(context.Background())

		timer := time.After(20 * time.Second)
		var wgDone sync.WaitGroup
		done := make(chan bool)
		go func() {
			<-time.After(500 * time.Millisecond)
			//log.Println("waiting for wgDone")
			wgDone.Wait()
			done <- true
			//log.Println("sent 'done'")
		}()

		generatedDocuments := make(chan bson.M, 100)
		for cur.Next(context.Background()) {
			record := bson.M{}
			if err := cur.Decode(&record); err != nil {
				log.Println(err)
				log.Fatal("Failure at decoding document")
			}
			//log.Println("got record", record)
			go func() {
				wgDone.Add(1)
				//log.Println("done++")
				defer wgDone.Done()
				//defer func() {
				//	log.Println("finished one")
				//}()
				foreignDocs := make(map[string]*bson.M)
				// get foreign docs
				for key, value := range record {
					if strings.HasPrefix(key, "FK_") {
						name := key[3:]
						foreignCollection := db.Collection(name)
						var id primitive.ObjectID
						switch vv := value.(type) {
						case string:
							id, err = primitive.ObjectIDFromHex(vv)
							if err != nil {
								log.Println(err)
								// skipping this doc
								continue
							}
						case primitive.ObjectID:
							// TODO: see if this or string
							id = vv
						default:
							log.Printf("%v has type %T and value %v", key, value, value)
							log.Fatal("don't know that type, pls fix")
						}
						res := bson.M{}
						if err := foreignCollection.FindOne(context.TODO(), bson.M{"_id": id}).Decode(&res); err != nil {
							// problem finding reffed foreign document
							log.Println(err)
						}
						//log.Println("got doc: ", res)
						//log.Println("name: ", name)
						foreignDocs[name] = &res
					} else {
						// TODO: handle manual fields?
						//log.Println("got other field: ", key)
					}
				}
				//log.Println("got fDocs ", foreignDocs)

				conv := make(map[string]interface{})
				for _, field := range tableForm.Fields {
					gotField := false
					for _, p := range field.InheritPatterns {
						values := make([]interface{}, 0, len(p.Params))
						for _, param := range p.Params {
							collectionName, foreignField := SplitParam(param)
							foreignDoc, ok := foreignDocs[collectionName]
							if !ok {
								// This record doesn't have link to the collection
								//log.Println("This record doesn't have link to the collection")
								break
							}
							foreignFieldValue, ok := (*foreignDoc)[foreignField]
							if !ok {
								// Foreign document doesn't have the specified field
								//log.Println("Foreign document doesn't have the specified field")
								break
							}
							values = append(values, foreignFieldValue)
						}
						if len(values) != len(p.Params) {
							log.Println("len", values, p.Params)
							continue
						}
						conv[field.Name] = fmt.Sprintf(p.Format, values...)
						gotField = true
						break
					}
					if !gotField {
						log.Println("didn't get the specified field:", field)
						// TODO: didn't get the specified field, do sth, set nil?
					}
				}
				generatedDocuments <- conv
				//log.Println("sent", conv)
			}()
		}
		go func() {
			select {
			case <-done:
				fmt.Println("done, closing chan")
			case <-timer:
				fmt.Println("timeout, closing chan")
			}
			close(generatedDocuments)
		}()
		genDocsArray := make([]bson.M, 0, 20)
		for doc := range generatedDocuments {
			genDocsArray = append(genDocsArray, doc)
		}
		c.JSON(http.StatusOK, genDocsArray)
		return
	})

	r.POST("/table/:name/insert", func(c *gin.Context) {
		name := c.Params.ByName("name")
		if !hasCollections(db, name) {
			c.String(http.StatusBadRequest, "Failure")
		}
		collection := db.Collection(name + "_manual")
		var items []interface{}
		if err := c.ShouldBind(&items); err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure")
			return
		}
		many, err := collection.InsertMany(context.TODO(), items)
		if err != nil {
			log.Println(err)
			c.String(http.StatusBadRequest, "Failure")
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "Success", "insertedIDs": many.InsertedIDs})
		c.String(http.StatusOK, "Success")
		return
	})

	//// Get user value
	//r.GET("/user/:name", func(c *gin.Context) {
	//	user := c.Params.ByName("name")
	//	value, ok := db[user]
	//	if ok {
	//		c.JSON(http.StatusOK, gin.H{"user": user, "value": value})
	//	} else {
	//		c.JSON(http.StatusOK, gin.H{"user": user, "status": "no value"})
	//	}
	//})
	//
	//// Authorized group (uses gin.BasicAuth() middleware)
	//// Same than:
	//// authorized := r.Group("/")
	//// authorized.Use(gin.BasicAuth(gin.Credentials{
	////	  "foo":  "bar",
	////	  "manu": "123",
	////}))
	//authorized := r.Group("/", gin.BasicAuth(gin.Accounts{
	//	"foo":  "bar", // user:foo password:bar
	//	"manu": "123", // user:manu password:123
	//}))
	//
	///* example curl for /admin with basicauth header
	//   Zm9vOmJhcg== is base64("foo:bar")
	//
	//	curl -X POST \
	//  	http://localhost:8080/admin \
	//  	-H 'authorization: Basic Zm9vOmJhcg==' \
	//  	-H 'content-type: application/json' \
	//  	-d '{"value":"bar"}'
	//*/
	//authorized.POST("admin", func(c *gin.Context) {
	//	user := c.MustGet(gin.AuthUserKey).(string)
	//
	//	// Parse JSON
	//	var json struct {
	//		Value string `json:"value" binding:"required"`
	//	}
	//
	//	if c.Bind(&json) == nil {
	//		db[user] = json.Value
	//		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	//	}
	//})

	return r
}

type CentralTable struct {
	Name   string
	Fields []Field

	DisplayName string
	Description string
}

type CentralTableData struct {
	Links map[string]primitive.ObjectID
}

//func test() {
//
//	var table = CentralTable{
//		Name: "tasty",
//		Fields: []Field{
//			{
//				Name:      "video",
//				FieldType: "URLField",
//				InheritPatterns: []InheritPattern{
//					{
//						"%v",
//						[]string{"tasty_manual.video"},
//					},
//				},
//			},
//			{
//				Name:      "id",
//				FieldType: "IntField",
//				InheritPatterns: []InheritPattern{
//					{
//						"%v_%v",
//						[]string{"movieDetails.actors", "movieDetails.title"},
//					},
//					{
//						"%v",
//						[]string{"tasty_manual.id"},
//					},
//				},
//			},
//		},
//	}
//	fmt.Print(table)
//}
