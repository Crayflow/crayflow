package condition

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestCompute(t *testing.T) {
	convey.Convey("test condition compute", t, func() {
		convey.Convey("condition invalid", func() {
			c := "'a' + 3"
			output, err := Compute(c, nil)
			_, _ = convey.Println(output, err)
			convey.So(err, convey.ShouldBeError)
			convey.So(output, convey.ShouldBeFalse)
		})

		convey.Convey("condition output invalid", func() {
			c := "'a' + 'b'"
			output, err := Compute(c, nil)
			_, _ = convey.Println(output, err)
			convey.So(err, convey.ShouldResemble, ErrConditionOutputInvalid)
			convey.So(output, convey.ShouldBeFalse)
		})

		convey.Convey("condition compute success", func() {
			c := `len('abc') == 3`
			output, err := Compute(c, nil)
			_, _ = convey.Println(output, err)
			convey.So(err, convey.ShouldBeNil)
			convey.So(output, convey.ShouldBeTrue)

			c = `len('abc') != 3`
			output, err = Compute(c, nil)
			_, _ = convey.Println(output, err)
			convey.So(err, convey.ShouldBeNil)
			convey.So(output, convey.ShouldBeFalse)
		})

		convey.Convey("condition compute with data", func() {
			type render struct {
				Envs map[string]string
				Keys map[string]string
			}
			data := render{
				Envs: map[string]string{
					"Key": "Value",
					"A":   "B",
				},
			}

			c1 := `Envs['Key'] == 'Value'`
			output, err := Compute(c1, data)
			_, _ = convey.Println(output, err)
			convey.So(err, convey.ShouldBeNil)
			convey.So(output, convey.ShouldBeTrue)

			c2 := `Envs['A'] != 'B'`
			output, err = Compute(c2, data)
			_, _ = convey.Println(output, err)
			convey.So(err, convey.ShouldBeNil)
			convey.So(output, convey.ShouldBeFalse)
		})
	})
}
