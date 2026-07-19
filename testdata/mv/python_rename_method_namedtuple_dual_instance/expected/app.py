from collections import namedtuple


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


Box = namedtuple("Box", ["a"])


def use_pos():
    ba = Box(A())
    bb = Box(B())
    return ba.a.execute() + bb.a.run()


def use_pos_var():
    ba = Box(A())
    bb = Box(B())
    xa = ba.a
    xb = bb.a
    return xa.execute() + xb.run()


def use_pos_index():
    ba = Box(A())
    bb = Box(B())
    return ba[0].execute() + bb[0].run()


def use_kw():
    ba = Box(a=A())
    bb = Box(a=B())
    return ba.a.execute() + bb.a.run()


def use_preserves_b():
    bb = Box(B())
    return bb.a.run()
