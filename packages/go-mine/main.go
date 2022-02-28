package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"matchmaker/henle"
	"matchmaker/server"
	"os"
	"strconv"
	"time"
)

func scrapeToFile() {
	// Create output file
	outFile, err := os.Create("output.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()

	henle.Scrape("csv", 1, outFile, nil)
}

func scrapeToStdout() {
	henle.Scrape("json", 0, os.Stdout, nil)
}


type Piece struct {
	Title []string
	Composer string
}

type HenlePiece struct {
	Title string
	Composer string
	Difficulty string
}

type PianoSyllabusPiece struct {
	URL string
	Composer string
	Title string
	ID string
	Grade string
	Syllabus string
	Youtube string
	Notes string
}

type PianoStreetPiece struct {
	URL string
	Composer string
	Title string
	Key string
	Type string
	Level string
	Notes string
}

type IMSLPPiece struct {
	URL string
	Title string
	Composer string
	HeaderInfo map[string]interface{}
	//Performance []IMSLPPerformce
	//SheetMusic []IMSLPSheetMusic
	GeneralInfo map[string]interface{}
}

type IMSLPComposer struct {
	URL string
}

func testMongo() {
	var a = map[string]interface{}{"a": 3, "role": "archer", "b": []int{1, 2, 3}}
	fmt.Println(a)

	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("test_database")
	collection := db.Collection("movieDetails")

	//var rest struct{director string}
	//res := c.FindOne(context.TODO(), bson.M{"year": int32(2013)}).Decode(&rest)
	//fmt.Println(res)
	obj, _ := primitive.ObjectIDFromHex("5e854b309cd55cbaf52c03fe")
	cur, err := collection.Find(context.Background(), bson.M{"_id": obj})
	if err != nil { log.Fatal(err) }
	defer cur.Close(context.Background())
	for cur.Next(context.Background()) {
		// To decode into a struct, use cursor.Decode()
		//result := struct{
		//	director string
		//	year int32
		//}{}
		result := map[string]interface{}{}
		err := cur.Decode(&result)
		if err != nil { log.Fatal(err) }
		// do something with result...
		fmt.Println(result)
		for k, v := range result {
			switch vv := v.(type) {
			//case string:
			//	fmt.Println(k, "is string", vv)
			//case float64:
			//	fmt.Println(k, "is float64", vv)
			//case []interface{}:
			//	fmt.Println(k, "is an array:")
			//	for i, u := range vv {
			//		fmt.Println(i, u)
			//	}
			default:
				fmt.Println(k, "__ is of a type I don't know how to handle")
				fmt.Printf("Type = %T, value = %v\n", vv, vv)
			}
		}

		// To get the raw bson bytes use cursor.Current
		//raw := cur.Current
		// do something with raw...
	}
	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}
}

func readMsczFiles(filename string, collection *mongo.Collection) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, _ := reader.ReadAll()
	rows := make([]interface{}, 0, len(records))
	for i, record := range records {
		if i == 0 {
			continue
		}
		id, _ := strconv.ParseInt(record[0], 10, 0)
		rows = append(rows, bson.M{"id": int(id), "ref": record[1]})
	}
	_, err = collection.InsertMany(context.TODO(), rows)
	return err
}

func readGradedPiecesAll(filename string, collection *mongo.Collection) error {
	file, err := os.Open(filename)
	if err != nil {
		log.Println(err)
		return err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, _ := reader.ReadAll()
	var headers []string
	for i, record := range records {
		if i == 0 {
			copy(headers, record)
		}
		v1, _ := strconv.ParseInt(record[0], 10, 0)
		v2, _ := strconv.ParseInt(record[1], 10, 0)
		res, err := collection.InsertOne(context.Background(), bson.M{
			"OrSor": int(v1),
			"Grade": int(v2),
			"Composer": record[2],
			"Composition": record[3],
			"Main Technical Difficulty or Benefit": record[4],
			"Other Notes & Comments": record[5],
		})
		if err != nil {
			log.Println(err)
		} else {
			fmt.Println("Inserted ID:", res.InsertedID)
		}
	}
	return err
}


func readJSONL(filename string, collection *mongo.Collection) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		var f interface{}
		if err := json.Unmarshal(scanner.Bytes(), &f); err != nil {
			return err
		}
		if res, err := collection.InsertOne(context.Background(), f); err != nil {
			log.Println(err)
		} else {
			fmt.Println("Inserted ID:", res.InsertedID)
		}
	}
	return nil
}

func main() {
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("test_database")

	//readJSONL("D:\\data\\MDC\\score.jsonl", db.Collection("score"))
	//readMsczFiles("D:\\data\\MDC\\mscz-files.csv", db.Collection("msczFiles"))
	//readGradedPiecesAll("D:\\data\\MDC\\Graded_Pieces_All.csv", db.Collection("pianoStreetPiece"))

	// Run server
	server.SetDatabase(db)
	r := server.SetupRouter()
	// Listen and Server in 0.0.0.0:8080
	r.Run(":8080")

	//henle.Scrape("mongo",1, nil, db.Collection("henleBooks"))
}
