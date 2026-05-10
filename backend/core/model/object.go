package model

import (
	"time"
)

// ObjWrapName decorates an Obj with an alternative display name —
// used when the underlying Obj's Name doesn't match the path
// (renames-in-flight, encoded paths).
type ObjWrapName struct {
	Name string
	Obj
}

func (o *ObjWrapName) Unwrap() Obj {
	return o.Obj
}

func (o *ObjWrapName) GetName() string {
	if o.Name == "" {
		o.Name = o.Obj.GetName()
	}
	return o.Name
}

// Object is the canonical concrete Obj. Drivers return Object (or
// one of the embed-extension types below) from their list/get
// calls. The Obj interface methods are satisfied by the GetX +
// ModTime + IsDir methods on this type.
type Object struct {
	ID       string
	Path     string
	Name     string
	Size     int64
	Modified time.Time
	IsFolder bool
}

func (o *Object) GetName() string {
	return o.Name
}

func (o *Object) GetSize() int64 {
	return o.Size
}

func (o *Object) ModTime() time.Time {
	return o.Modified
}

func (o *Object) IsDir() bool {
	return o.IsFolder
}

func (o *Object) GetID() string {
	return o.ID
}

func (o *Object) GetPath() string {
	return o.Path
}

func (o *Object) SetPath(id string) {
	o.Path = id
}

// Thumbnail is the optional embed for Obj implementations that
// can produce a thumbnail URL — drives the file-browser preview.
type Thumbnail struct {
	Thumbnail string
}

// Url is the optional embed for Obj implementations that can
// produce a direct download URL (302-friendly storage backends).
type Url struct {
	Url string
}

func (w Url) URL() string {
	return w.Url
}

func (t Thumbnail) Thumb() string {
	return t.Thumbnail
}

// ObjThumb is Object + thumbnail support.
type ObjThumb struct {
	Object
	Thumbnail
}

// ObjectURL is Object + direct-download URL support.
type ObjectURL struct {
	Object
	Url
}

// ObjThumbURL is Object + thumbnail + direct-download URL.
type ObjThumbURL struct {
	Object
	Thumbnail
	Url
}
