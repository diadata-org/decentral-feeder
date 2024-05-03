package utils

import (
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// GetRequest performs a get request on @url and returns the response body
// as a slice of byte data.
func GetRequest(url string) ([]byte, int, error) {

	response, err := http.Get(url) //nolint:noctx,gosec
	if err != nil {
		log.Error("get request: ", err)
		return []byte{}, 0, err
	}

	// Close response body after function
	defer func() {
		cerr := response.Body.Close()
		if err == nil {
			err = cerr
		}
	}()

	// Check the status code for a 200 so we know we have received a
	// proper response.
	if response.StatusCode != 200 {
		return []byte{}, response.StatusCode, fmt.Errorf("HTTP Response Error %d", response.StatusCode)
	}

	// Read the response body
	XMLdata, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error(err)
		return []byte{}, response.StatusCode, err
	}

	return XMLdata, response.StatusCode, err
}
