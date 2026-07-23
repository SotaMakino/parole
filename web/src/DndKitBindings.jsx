// Thin wrappers around @dnd-kit so ReScript can drive it: dnd-kit exposes
// per-element hooks whose `attributes`/`listeners` are meant to be spread onto
// the DOM node, which ReScript's JSX can't express directly.
import {
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  PointerSensor,
} from "@dnd-kit/core";
import { CSS } from "@dnd-kit/utilities";
import * as React from "react";

// A keyboard letter you can pick up. `disabled` greys the button (letter fully
// placed); `dragDisabled` blocks dragging but still allows the tap-to-select.
export function Draggable({ letter, label, className, disabled, dragDisabled, onClick }) {
  const { attributes, listeners, setNodeRef, transform } = useDraggable({
    id: letter,
    disabled: disabled || dragDisabled,
  });
  const style = transform
    ? { transform: CSS.Translate.toString(transform), zIndex: 20 }
    : undefined;
  return React.createElement(
    "button",
    {
      ref: setNodeRef,
      type: "button",
      className,
      disabled,
      onClick,
      style,
      ...attributes,
      ...listeners,
    },
    label,
  );
}

// An open tile that accepts a dropped letter. `armed` means a letter is in hand,
// so gate the hover highlight on it to match the tap-to-place styling.
export function Droppable({ dropId, className, armed, onClick }) {
  const { setNodeRef, isOver } = useDroppable({ id: dropId });
  const cls = className + (isOver && armed ? " drop-hover" : "");
  return React.createElement("div", { ref: setNodeRef, className: cls, onClick });
}

// A small activation distance lets a plain tap fire onClick instead of a drag,
// so tap-to-select keeps working alongside drag-and-drop.
export function useDefaultSensors() {
  return useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }));
}
