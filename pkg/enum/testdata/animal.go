package testdata

//go:generate toolx gen enum Animal

type Animal int

const (
	ANIMAL__DOG  Animal = iota // 狗
	ANIMAL__CAT                // 猫
	ANIMAL__FISH               // 鱼
)
