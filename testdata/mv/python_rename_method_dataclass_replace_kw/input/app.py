from dataclasses import dataclass, replace


@dataclass
class Box:
    a: object


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_replace_kw():
    ba = Box(B())
    bb = Box(A())
    return replace(ba, a=A()).a.run() + replace(bb, a=B()).a.run()


def use_replace_kw_inline():
    return replace(Box(B()), a=A()).a.run() + replace(Box(A()), a=B()).a.run()


def use_preserves_b():
    return replace(Box(A()), a=B()).a.run()
