package seeder

import (
	"testing"
)

func TestCreateSeeder(t *testing.T) {
    returnValue := seeder(1)
    if returnValue != 0 {
        t.Errorf("seeder(1) = %d; should be 0!", returnValue)
    }
}
