package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	port string
)

type TrashCollection struct {
	Name string `json:"name"`
	//CollectionPeriod string
	CollectionDates []time.Time `json:"dates"`
}

func parseTrashCollection(s *goquery.Selection) (*TrashCollection, error) {
	nameAndPeriod := strings.Split(s.Children().Eq(1).Text(), ", ")
	if len(nameAndPeriod) < 1 {
		return nil, errors.New("Not a periodical trash collection.")
	}
	collectionDatesText := s.Children().Last().Text()
	dayRegex, _ := regexp.Compile("den (\\d+\\.\\d+\\.\\d+)")
	res := dayRegex.FindAllString(collectionDatesText, 100)
	collectionDates := []time.Time{}

	for _, date := range res {
		d, _ := time.Parse("02.01.2006", strings.Split(date, " ")[1])
		collectionDates = append(collectionDates, d)
	}
	t := TrashCollection{
		Name:            nameAndPeriod[0],
		CollectionDates: collectionDates,
	}
	return &t, nil
}

func collectionForAddress(street string, houseNr int) *[]TrashCollection {
	requestUrl := fmt.Sprintf("https://web6.karlsruhe.de/service/abfall/akal/akal.php?strasse=%s&hausnr=%d", street, houseNr)
	res, err := http.Get(requestUrl)
	if err != nil || res.StatusCode != 200 {
		panic("Error request")
	}
	defer res.Body.Close()

	collections := []TrashCollection{}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	doc.Find("#nfoo>.row").Each(func(i int, s *goquery.Selection) {
		if s.Children().Length() == 3 {
			collection, err := parseTrashCollection(s)
			if err == nil && len(collection.CollectionDates) > 0 {
				collections = append(collections, *collection)
			}
		}
	})
	return &collections
}

func main() {

	flag.StringVar(&port, "port", ":7295", "Port")
	flag.Parse()

	http.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		street := req.URL.Query().Get("street")
		nr := req.URL.Query().Get("nr")

		if street == "" || nr == "" {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}

		nrParsed, err := strconv.Atoi(nr)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		collection := *collectionForAddress(street, nrParsed)
		wrappedCollectionDates := struct {
			CollectionDates []TrashCollection `json:"CollectionDates"`
		}{
			CollectionDates: collection,
		}
		collectionJson, err := json.Marshal(wrappedCollectionDates)
		if err != nil {
			io.WriteString(resp, err.Error())
			return
		}
		resp.Header().Add("Access-Control-Allow-Origin", "*")
		resp.Write(collectionJson)
	})
	http.ListenAndServe(port, nil)
}
