// Adds drag-over reporting to GLFW's content view (see drag_darwin.go).

#import <Cocoa/Cocoa.h>
#include <objc/runtime.h>
#include "_cgo_export.h"

// The position is computed exactly as GLFW's performDragOperation: does,
// so the hover point and the drop point agree.
static NSDragOperation wav404DraggingUpdated(id self, SEL cmd, id<NSDraggingInfo> sender) {
    const NSRect contentRect = [(NSView *)self frame];
    const NSPoint pos = [sender draggingLocation];
    wav404DragMoved(pos.x, contentRect.size.height - pos.y);
    return NSDragOperationCopy;
}

static void wav404DraggingGone(id self, SEL cmd, id<NSDraggingInfo> sender) {
    wav404DragGone();
}

void wav404InstallDragHook(void) {
    Class view = objc_getClass("GLFWContentView");
    if (view == Nil) {
        return;
    }
    // class_addMethod leaves a method the class already has untouched.
    class_addMethod(view, @selector(draggingUpdated:), (IMP)wav404DraggingUpdated, "Q@:@");
    class_addMethod(view, @selector(draggingExited:), (IMP)wav404DraggingGone, "v@:@");
    class_addMethod(view, @selector(draggingEnded:), (IMP)wav404DraggingGone, "v@:@");
}
