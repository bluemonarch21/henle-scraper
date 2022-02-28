package server

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)


func strListRemove(s []string, i int) []string {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

func strListContains(arr []string, elems ...string) bool {
	for _, e := range arr {
		for i, elem := range elems {
			if e == elem {
				elems = strListRemove(elems, i)
				break
			}
		}
	}
	return len(elems) == 0
}

func hasCollections(db *mongo.Database, names ...string) bool {
	cnames, _ := db.ListCollectionNames(context.TODO(), bson.M{})
	//log.Println(cnames)
	//log.Println(names)
	return strListContains(cnames, names...)
}

