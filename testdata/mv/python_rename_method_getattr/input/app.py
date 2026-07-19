from dataclasses import dataclass


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Box:
    a: A
    b: B


def use_getattr(box: Box) -> int:
    xa = getattr(box, "a")
    xb = getattr(box, "b")
    return xa.run() + xb.run()


def use_chain(box: Box) -> int:
    return getattr(box, "a").run() + getattr(box, "b").run()


def use_walrus(box: Box) -> int:
    if (xa := getattr(box, "a")):
        return xa.run()
    return 0
