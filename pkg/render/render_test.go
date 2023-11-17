package render

import (
	"encoding/json"
	devopsv1 "github.com/buhuipao/crayflow/api/v1"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func Test_instanceRender_Do(t *testing.T) {
	convey.Convey("测试数据渲染", t, func() {
		strVar := "Value"

		intVar := 123
		bs1, _ := json.Marshal(intVar)

		listVar := []string{"1.2.3", "4.5.6"}
		bs2, _ := json.Marshal(listVar)

		mapVar := map[string]interface{}{
			"x": float64(1),
			"y": []interface{}{"123"},
			"z": "xxx",
		}
		bs3, _ := json.Marshal(mapVar)

		data := devopsv1.Workflow{
			Status: devopsv1.WorkflowStatus{
				Phase: devopsv1.WorkflowPhaseRunning,
				Vars: []devopsv1.WorkflowVariableSpec{
					{
						Key:   "STRING_VAR",
						Value: strVar,
					},
					{
						Key:   "INT_VAR",
						Value: string(bs1),
					},
					{
						Key:   "LIST_VAR",
						Value: string(bs2),
					},
					{
						Key:   "MAP_VAR",
						Value: string(bs3),
					},
				},
			},
		}
		render := New(&data)

		bs, _ := json.MarshalIndent(data, "", "\t")
		_, _ = convey.Println(string(bs))

		convey.Convey("测试字符串变量渲染", func() {
			inputs, _ := json.Marshal(map[string]interface{}{
				"Key": "{{ .Vars.STRING_VAR }}",
			})
			actualInput, err := render.Do(string(inputs))
			_, _ = convey.Println("rendered inputs: ", actualInput, "error:", err)

			convey.So(err, convey.ShouldBeNil)
			convey.So(actualInput, convey.ShouldEqual, `{"Key":"Value"}`)
		})

		convey.Convey("测试非字符串变量渲染", func() {
			template := map[string]interface{}{
				"String": "{{ .Vars.STRING_VAR }}",
				"Int":    "|{{ .Vars.INT_VAR }}|",
				"List":   "|{{ .Vars.LIST_VAR }}|",
				"Map":    "|{{ .Vars.MAP_VAR }}|",
			}
			bs, _ := json.MarshalIndent(template, "", "\t")
			_, _ = convey.Println(string(bs))

			actualInput, err := render.Do(string(bs))
			_, _ = convey.Println("rendered inputs: ", actualInput, "error:", err)
			convey.So(err, convey.ShouldBeNil)

			type T struct {
				String string
				Int    int
				List   []string
				Map    map[string]interface{}
			}
			var t T
			err = json.Unmarshal([]byte(actualInput), &t)
			_, _ = convey.Printf("rendered data: %#v\n", t)

			convey.So(err, convey.ShouldBeNil)
			convey.So(t.String, convey.ShouldEqual, strVar)
			convey.So(t.Int, convey.ShouldEqual, intVar)
			convey.So(t.List, convey.ShouldResemble, listVar)
			convey.So(t.Map, convey.ShouldResemble, mapVar)
		})
	})
}
