from operator import attrgetter
from dataclasses import dataclass


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Box:
    a: A


@dataclass
class BoxB:
    a: B


class Wrap:
    item: Box

    def __init__(self, item: Box):
        self.item = item

    def get(self) -> Box:
        return self.item


class WrapB:
    item: BoxB

    def __init__(self, item: BoxB):
        self.item = item

    def get(self) -> BoxB:
        return self.item


def use_attrgetter(w: Wrap, wb: WrapB) -> int:
    return attrgetter("a")(w.get()).execute() + attrgetter("a")(wb.get()).run()


def use_attrgetter_assign(w: Wrap, wb: WrapB) -> int:
    xa = attrgetter("a")(w.get())
    xb = attrgetter("a")(wb.get())
    return xa.execute() + xb.run()


def use_getattr(w: Wrap, wb: WrapB) -> int:
    return getattr(w.get(), "a").execute() + getattr(wb.get(), "a").run()


def use_getattr_assign(w: Wrap, wb: WrapB) -> int:
    xa = getattr(w.get(), "a")
    xb = getattr(wb.get(), "a")
    return xa.execute() + xb.run()


def use_class() -> int:
    return attrgetter("a")(Box(A())).execute() + attrgetter("a")(BoxB(B())).run()


def use_typed(box: Box, boxb: BoxB) -> int:
    return attrgetter("a")(box).execute() + getattr(boxb, "a").run()


def use_preserves_b(wb: WrapB) -> int:
    return attrgetter("a")(wb.get()).run() + getattr(wb.get(), "a").run()
