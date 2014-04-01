package gosegment

import ()

func NewSegment(dir string) (seg *Segment, err error) {
	seg = &Segment{}
	err = seg.Init(dir)
	return
}
