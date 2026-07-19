from contextlib import nullcontext, closing
import contextlib


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    def __init__(self) -> None:
        self.held = A()

    def get(self) -> A:
        return self.held


class BoxB:
    def __init__(self) -> None:
        self.held = B()

    def get(self) -> B:
        return self.held


def use_nullcontext_mr(ba: BoxA, bb: BoxB):
    with nullcontext(ba.get()) as xa:
        with nullcontext(bb.get()) as xb:
            return xa.execute() + xb.run()


def use_contextlib_nullcontext_mr(ba: BoxA, bb: BoxB):
    with contextlib.nullcontext(ba.get()) as xa:
        with contextlib.nullcontext(bb.get()) as xb:
            return xa.execute() + xb.run()


def use_closing_mr(ba: BoxA, bb: BoxB):
    with closing(ba.get()) as xa:
        with closing(bb.get()) as xb:
            return xa.execute() + xb.run()


def use_contextlib_closing_mr(ba: BoxA, bb: BoxB):
    with contextlib.closing(ba.get()) as xa:
        with contextlib.closing(bb.get()) as xb:
            return xa.execute() + xb.run()


# Class regression — already worked.
def use_class():
    with nullcontext(A()) as xa:
        with nullcontext(B()) as xb:
            return xa.execute() + xb.run()


def use_closing_class():
    with closing(A()) as xa:
        with closing(B()) as xb:
            return xa.execute() + xb.run()


def use_preserves_b(bb: BoxB):
    with nullcontext(bb.get()) as xb:
        n = xb.run()
    with nullcontext(B()) as xb2:
        n += xb2.run()
    return n
