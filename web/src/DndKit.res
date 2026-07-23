// ReScript bindings for @dnd-kit/core plus the local DndKitBindings.jsx shim.
type sensors

@module("./DndKitBindings.jsx")
external useDefaultSensors: unit => sensors = "useDefaultSensors"

// dnd-kit hands each callback an event; ids are the strings we pass in
// (letters for draggables, "wordIndex-position" for droppables).
type identified = {id: string}
type dragStartEvent = {active: identified}
type dragEndEvent = {active: identified, over: Js.Nullable.t<identified>}

module DndContext = {
  @module("@dnd-kit/core") @react.component
  external make: (
    ~sensors: sensors=?,
    ~onDragStart: dragStartEvent => unit=?,
    ~onDragEnd: dragEndEvent => unit=?,
    ~onDragCancel: unit => unit=?,
    ~children: React.element,
  ) => React.element = "DndContext"
}

module Draggable = {
  @module("./DndKitBindings.jsx") @react.component
  external make: (
    ~letter: string,
    ~label: string,
    ~className: string,
    ~disabled: bool=?,
    ~dragDisabled: bool=?,
    ~onClick: ReactEvent.Mouse.t => unit=?,
  ) => React.element = "Draggable"
}

module Droppable = {
  @module("./DndKitBindings.jsx") @react.component
  external make: (
    ~dropId: string,
    ~className: string,
    ~armed: bool,
    ~onClick: ReactEvent.Mouse.t => unit=?,
  ) => React.element = "Droppable"
}
