from typing import NamedTuple


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


Box = NamedTuple("Box", [("a", A), ("b", B)])
KwBox = NamedTuple("KwBox", a=A, b=B)


def use_direct(box: Box) -> int:
    return box.a.execute() + box.b.run()


def use_var(box: Box) -> int:
    xa = box.a
    xb = box.b
    return xa.execute() + xb.run()


def use_kw(box: KwBox) -> int:
    return box.a.execute() + box.b.run()


def use_preserves_b(box: Box) -> int:
    return box.b.run()
