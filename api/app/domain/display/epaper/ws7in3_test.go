package epaper

// func TestWS7in3FFramer(t *testing.T) {
// 	const deviceWidth = 800
// 	const deviceHeight = 480

// 	var tests = []struct {
// 		imgf string
// 		want uint8
// 	}{
// 		{"testdata/xga.png", uint8(0)}, // 0000 0000
// 	}

// 	sut := NewWS7in3F()

// 	for _, tt := range tests {
// 		// t.Run enables running "subtests", one for each
// 		// table entry. These are shown separately
// 		// when executing `go test -v`.
// 		testname := tt.imgf
// 		t.Run(testname, func(t *testing.T) {
// 			imgf, err := os.Open(tt.imgf)
// 			if err != nil {
// 				t.Fatal("file open error")
// 			}
// 			img, _, err := image.Decode(imgf)
// 			if err != nil {
// 				t.Fatal("decode error")
// 			}

// 			img2 := sut.Resize(sut.Crop(img))
// 			bounds := img2.Bounds()

// 			assert.Equal(t, deviceWidth, bounds.Max.X)
// 			assert.Equal(t, deviceHeight, bounds.Max.Y)
// 		})
// 	}

// }

// func TestWS7in3EFramer(t *testing.T) {
// 	const deviceWidth = 800
// 	const deviceHeight = 480

// 	var tests = []struct {
// 		imgf string
// 		want uint8
// 	}{
// 		{"testdata/xga.png", uint8(0)}, // 0000 0000
// 	}

// 	sut := NewWS7in3E()

// 	for _, tt := range tests {
// 		// t.Run enables running "subtests", one for each
// 		// table entry. These are shown separately
// 		// when executing `go test -v`.
// 		testname := tt.imgf
// 		t.Run(testname, func(t *testing.T) {
// 			imgf, err := os.Open(tt.imgf)
// 			if err != nil {
// 				t.Fatal("file open error")
// 			}
// 			img, _, err := image.Decode(imgf)
// 			if err != nil {
// 				t.Fatal("decode error")
// 			}

// 			img2 := sut.Resize(sut.Crop(img))
// 			bounds := img2.Bounds()

// 			assert.Equal(t, deviceWidth, bounds.Max.X)
// 			assert.Equal(t, deviceHeight, bounds.Max.Y)
// 		})
// 	}

// }
