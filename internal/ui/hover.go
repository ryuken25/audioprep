package ui

import "fyne.io/fyne/v2/driver/desktop"

// These satisfy desktop.Hoverable so the drop zone lights up under the mouse.
// Kept in their own file because they pull in the desktop driver package.

func (d *DropZone) MouseIn(_ *desktop.MouseEvent)    { d.setHover(true) }
func (d *DropZone) MouseMoved(_ *desktop.MouseEvent) {}
func (d *DropZone) MouseOut()                        { d.setHover(false) }
