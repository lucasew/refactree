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


def use_direct(box: Box) -> int:
    return box.a.run() + box.b.run()


def use_var(box: Box) -> int:
    xa = box.a
    xb = box.b
    return xa.run() + xb.run()


def use_preserves_b(box: Box) -> int:
    return box.b.run()
