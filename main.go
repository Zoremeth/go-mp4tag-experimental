package main

import (
	"fmt"
	//"os"
	"main/mp4tag"

)

func main() {
	mp4File, err := mp4tag.Open("1.m4a")
	if err != nil {
		panic(err)
	}
	defer mp4File.Close()
	parsedTags, err := mp4File.Read()
	if err != nil {
		panic(err)
	}
	fmt.Println("Album:", parsedTags.Album)
	fmt.Println("Artist:", parsedTags.Artist)
	fmt.Println("AlbumArtist:", parsedTags.AlbumArtist)
	fmt.Println("Comment:", parsedTags.Comment)
	fmt.Println("Composer:", parsedTags.Composer)
	fmt.Println("Genre:", parsedTags.Genre)
	fmt.Println("Title:", parsedTags.Title)
	fmt.Println("TrackNumber:", parsedTags.TrackNumber)
	fmt.Println("TrackTotal:", parsedTags.TrackTotal)
	fmt.Println("Year:", parsedTags.Year)
	fmt.Println("Custom:")
	for k, v := range parsedTags.Custom {
		fmt.Println(k + ":", v)
	}
	fmt.Println("Covers count:", len(parsedTags.CoversData))

	// coverData, _ := os.ReadFile("1.png")
	// coverData2, _ := os.ReadFile("2.png")

	// tags := &mp4tag.Tags{
	// 	//CoversData: [][]byte{coverData, coverData2},
	// 	Year: 2001,
	// 	Delete: []string{"compilation"},
	// }
	// err = mp4File.Write(tags)
	// if err != nil {
	// 	panic(err)
	// }
}