# screencage

A graphical screen capture tool that uses the
window position and size to select part of the screen to
capture.

![1FPS demo](demo.gif)
**Note: demo is 1 FPS to save file size**

## Primary packages/libraries used

- [ebiten](https://github.com/hajimehoshi/ebiten) - a game engine
- [screenshot](https://github.com/kbinani/screenshot) - a screenshot library
- [gif](https://github.com/nvlled/gogif) - a modified image/gif that allows streaming
- [dialog](https://github.com/sqweek/dialog) - cross-platform dialogs
- [carrot](https://github.com/nvlled/carrot) - a coroutine library

## Installation

This is supposed to be a personal tool for me, but if you
want to try it out, you can do the following:

`go install github.com/nvlled/screencage`

If you get some errors, check the system dependencies
for each packages/libraries listed above, and then retry again.
I'm a bit lazy right now, but I should directly list the dependencies here.

Note: tested only on Ubuntu/linux
