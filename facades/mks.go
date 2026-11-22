// Package facades gives a short way to resolve the panel from the container:
//
//	panel := facades.MKS()
package facades

import (
	"log"

	mks "github.com/MiranSalehi/mks"
)

// MKS returns the panel bound by mks.ServiceProvider.
func MKS() *mks.Panel {
	instance, err := mks.App.Make(mks.Binding)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	return instance.(*mks.Panel)
}
