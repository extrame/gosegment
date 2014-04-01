package gosegment

import (
	"container/list"
	"github.com/extrame/gosegment/dict"
	"github.com/extrame/gosegment/framework"
	"github.com/extrame/gosegment/match"
	"github.com/extrame/gosegment/utils"
	"regexp"
	"strings"
	"unicode"
)

const PATTERNS = `([０-９\d]+)|([ａ-ｚＡ-Ｚa-zA-Z_]+)`

type Segment struct {
	options        *match.MatchOptions
	params         *match.MatchParameter
	verbTable      map[string]string
	wordDictionary *dict.WordDictionary
	chsName        *dict.ChsName
	stopWord       *dict.StopWord
	synonym        *dict.Synonym
	re             *regexp.Regexp
}

func (s *Segment) Init(dictPath string) (err error) {
	s.re = regexp.MustCompile(PATTERNS)
	err = s.loadVerbTable(dictPath + "/Verbtable.txt")
	if err == nil {
		err = s.loadDictionary(dictPath)
	}
	return
}

func (s *Segment) loadVerbTable(file string) (err error) {
	s.verbTable = make(map[string]string)
	err = utils.EachLine(file, func(line string) {
		words := strings.Split(line, "\t")
		if len(words) == 3 {
			value := strings.TrimSpace(strings.ToLower(words[0]))
			for j := 1; j < 3; j++ {
				key := strings.TrimSpace(strings.ToLower(words[j]))
				s.verbTable[key] = value
			}
		}
	})
	return
}

func (s *Segment) loadDictionary(dictPath string) (err error) {
	s.wordDictionary = dict.NewWordDictionary()
	err = s.wordDictionary.Load(dictPath + "/Dict.txt")
	if err == nil {
		s.chsName = dict.NewChsName()
		s.wordDictionary.ChineseName = s.chsName
		err = s.chsName.Load(dictPath)
	}
	if err == nil {
		s.stopWord = dict.NewStopWord()
		err = s.stopWord.Load(dictPath + "/Stopword.txt")
	}
	if err == nil {
		s.synonym = dict.NewSynonym()
		err = s.synonym.Load(dictPath)
	}
	// todo: wildchar & segment cross referrence problem
	return
}

func (s *Segment) DoSegment(text string) *list.List {
	return s.DoSegmentWithOptionParam(text, nil, nil)
}

func (s *Segment) DoSegmentWithOption(text string, params *match.MatchOptions) *list.List {
	return s.DoSegmentWithOptionParam(text, params, nil)
}

func (s *Segment) DoSegmentWithOptionParam(text string, options *match.MatchOptions, params *match.MatchParameter) *list.List {

	if len(text) == 0 {
		return list.New()
	}

	s.options = options
	s.params = params

	if s.options == nil {
		s.options = match.NewMatchOptions()
	}

	if s.params == nil {
		s.params = match.NewMatchParameter()
	}

	result := s.preSegment(text)
	if s.options.FilterStopWords {
		s.filterStopWord(result)
	}
	s.processAfterSegment(text, result)

	return result
}

func (s *Segment) preSegment(text string) *list.List {
	result := s.getInitSegment(text)
	runes := utils.ToRunes(text)
	cur := result.Front()
	for cur != nil {
		if s.options.IgnoreSpace {
			if cur.Value.(*WordInfo).WordType == TSpace {
				lst := cur
				cur = cur.Next()
				result.Remove(lst)
				continue
			}
		}
		switch cur.Value.(*WordInfo).WordType {
		case TSimplifiedChinese:
			inputText := cur.Value.(*WordInfo).Word
			originalWordType := TSimplifiedChinese
			pls := s.wordDictionary.GetAllMatchs(inputText, s.options.ChineseNameIdentify)
			chsMatch := NewChsFullTextMatch(s.wordDictionary)
			chsMatch.SetOptionParams(s.options, s.params)
			chsMatchWords := chsMatch.Match(pls, inputText)
			curChsMatch := chsMatchWords.Front()
			for curChsMatch != nil {
				wi := curChsMatch.Value.(*WordInfo)
				wi.Position += cur.Value.(*WordInfo).Position
				wi.OriginalWordType = originalWordType
				wi.WordType = originalWordType
				curChsMatch = curChsMatch.Next()
			}
			rcur := utils.InsertAfterList(result, chsMatchWords, cur)
			removeItem := cur
			cur = rcur.Next()
			result.Remove(removeItem)
		case TEnglish:
			cur.Value.(*WordInfo).Rank = s.params.EnglishRank
			cur.Value.(*WordInfo).Word = s.convertChineseCapicalToAsiic(cur.Value.(*WordInfo).Word)
			if s.options.IgnoreCapital {
				cur.Value.(*WordInfo).Word = strings.ToLower(cur.Value.(*WordInfo).Word)
			}

			if s.options.EnglishSegment {
				lower := strings.ToLower(cur.Value.(*WordInfo).Word)
				if lower != cur.Value.(*WordInfo).Word {
					result.InsertBefore(NewWordInfo(lower, cur.Value.(*WordInfo).Position, dict.POS_A_NX, 1, s.params.EnglishLowerRank, TEnglish, TEnglish), cur)
				}
				stem := s.getStem(lower)
				if len(stem) > 0 {
					if lower != stem {
						result.InsertBefore(NewWordInfo(stem, cur.Value.(*WordInfo).Position, dict.POS_A_NX, 1, s.params.EnglishStemRank, TEnglish, TEnglish), cur)
					}
				}
			}

			if s.options.EnglishMultiDimensionality {
				needSplit := false
				for _, c := range cur.Value.(*WordInfo).Word {
					if (c >= '0' && c <= '9') || (c == '_') {
						needSplit = true
						break
					}
				}
				if needSplit {
					output := s.re.FindAllString(cur.Value.(*WordInfo).Word, -1)
					if len(output) > 1 {
						position := cur.Value.(*WordInfo).Position
						for _, splitWord := range output {
							if len(splitWord) == 0 {
								continue
							}

							var wi *WordInfo
							r := utils.FirstRune(splitWord)
							if r >= '0' && r <= '9' {
								wi = NewWordInfoSome(splitWord, dict.POS_A_M, 1)
								wi.Position = position
								wi.Rank = s.params.NumericRank
								wi.OriginalWordType = TEnglish
								wi.WordType = TNumeric
							} else {
								wi = NewWordInfoSome(splitWord, dict.POS_A_NX, 1)
								wi.Position = position
								wi.Rank = s.params.EnglishRank
								wi.OriginalWordType = TEnglish
								wi.WordType = TEnglish
							}

							result.InsertBefore(wi, cur)
							position += utils.RuneLen(splitWord)
						}
					}
				}
			}

			var ok bool
			if ok, cur = s.mergeEnglishSpecialWord(runes, result, cur); !ok {
				cur = cur.Next()
			}

		case TNumeric:
			cur.Value.(*WordInfo).Word = s.convertChineseCapicalToAsiic(cur.Value.(*WordInfo).Word)
			cur.Value.(*WordInfo).Rank = s.params.NumericRank
			var ok bool
			if ok, cur = s.mergeEnglishSpecialWord(runes, result, cur); !ok {
				cur = cur.Next()
			}
		case TSymbol:
			cur.Value.(*WordInfo).Rank = s.params.SymbolRank
			cur = cur.Next()
		default:
			cur = cur.Next()
		}
	}
	return result
}

func (s *Segment) getStem(word string) string {
	if stem, ok := s.verbTable[word]; ok {
		return stem
	}

	st := framework.NewStemmer()
	for _, r := range word {
		if unicode.IsLetter(r) {
			st.Add(r)
		}
	}
	st.Stem()

	return st.ToString()
}

func (s *Segment) mergeEnglishSpecialWord(orginalText []rune, wordInfoList *list.List, current *list.Element) (bool, *list.Element) {
	cur := current
	cur = cur.Next()

	last := -1
	for cur != nil {
		if cur.Value.(*WordInfo).WordType == TSymbol || cur.Value.(*WordInfo).WordType == TEnglish {
			last = cur.Value.(*WordInfo).Position + utils.RuneLen(cur.Value.(*WordInfo).Word)
			cur = cur.Next()
		} else {
			break
		}
	}

	if last >= 0 {
		first := current.Value.(*WordInfo).Position
		newWord := orginalText[first:last]
		wa := s.wordDictionary.GetWordAttr(newWord)
		if wa == nil {
			return false, current
		}

		for current != cur {
			removeItem := current
			current = current.Next()
			wordInfoList.Remove(removeItem)
		}

		wi := NewWordInfoDefault()
		wi.Word = string(newWord)
		wi.Pos = wa.Pos
		wi.Frequency = wa.Frequency
		wi.WordType = TEnglish
		wi.Position = first
		wi.Rank = s.params.EnglishRank

		if current == nil {
			wordInfoList.PushBack(wi)
		} else {
			wordInfoList.InsertBefore(wi, current)
		}

		return true, current
	}

	return false, current
}

func (s *Segment) convertChineseCapicalToAsiic(text string) string {
	runes := utils.ToRunes(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] >= '０' && runes[i] <= '９' {
			runes[i] -= '０'
			runes[i] += '0'
		} else if runes[i] >= 'ａ' && runes[i] <= 'ｚ' {
			runes[i] -= 'ａ'
			runes[i] += 'a'
		} else if runes[i] >= 'Ａ' && runes[i] <= 'Ｚ' {
			runes[i] -= 'Ａ'
			runes[i] += 'A'
		}
	}
	return string(runes)
}

func (s *Segment) getInitSegment(text string) *list.List {
	result := list.New()
	runes := utils.ToRunes(text)
	lexical := NewLexical(runes)
	var dfaResult int

	for i := 0; i < len(runes); i++ {
		dfaResult = lexical.Input(runes[i], i)
		switch dfaResult {
		case Continue:
			continue
		case Quit:
			result.PushBack(lexical.OutputToken)
		case ElseQuit:
			result.PushBack(lexical.OutputToken)
			if lexical.OldState != 255 {
				i--
			}
		}
	}

	dfaResult = lexical.Input(0, len(runes))
	switch dfaResult {
	case Continue:
	case Quit:
		result.PushBack(lexical.OutputToken)
	case ElseQuit:
		result.PushBack(lexical.OutputToken)
	}
	return result
}

func (s *Segment) filterStopWord(wordInfoList *list.List) {
	if wordInfoList == nil {
		return
	}
	cur := wordInfoList.Front()
	for cur != nil {
		if s.stopWord.IsStopWord(cur.Value.(*WordInfo).Word, s.options.FilterEnglish, s.params.FilterEnglishLength, s.options.FilterNumeric, s.params.FilterNumericLength) {
			remoteItem := cur
			cur = cur.Next()
			wordInfoList.Remove(remoteItem)
		} else {
			cur = cur.Next()
		}
	}
}

func (s *Segment) processAfterSegment(text string, result *list.List) {
	// 匹配同义词
	if s.options.SynonymOutput {
		node := result.Front()
		for node != nil {
			pW := node.Value.(*WordInfo)
			synonyms := s.synonym.GetSynonyms(pW.Word)
			if synonyms != nil {
				for _, word := range synonyms {
					node = result.InsertAfter(NewWordInfo(word, pW.Position, pW.Pos, pW.Frequency, s.params.SymbolRank, TSynonym, pW.WordType), node)
				}
			}
			node = node.Next()
		}
	}

	// 通配符匹配
	if s.options.WildcardOutput {
		// todo: >>>>>>>
	}

}
