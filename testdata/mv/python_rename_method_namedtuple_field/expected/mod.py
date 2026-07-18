from collections import namedtuple


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


Box = namedtuple("Box", ["a", "b"])


def use_direct() -> int:
    box = Box(A(), B())
    return box.a.execute() + box.b.run()


def use_var() -> int:
    box = Box(A(), B())
    xa = box.a
    xb = box.b
    return xa.execute() + xb.run()


def use_kw() -> int:
    box = Box(a=A(), b=B())
    return box.a.execute() + box.b.run()


def use_param(box: Box) -> int:
    return box.a.execute() + box.b.run()


def use_preserves_b() -> int:
    box = Box(A(), B())
    return box.b.run()
