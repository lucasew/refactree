from contextlib import nullcontext, closing
import contextlib


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_nullcontext(a: A, b: B):
    with nullcontext(a) as xa, nullcontext(b) as xb:
        return xa.run() + xb.run()


def use_closing():
    with closing(A()) as xa, closing(B()) as xb:
        return xa.run() + xb.run()


def use_contextlib():
    with contextlib.nullcontext(A()) as xa, contextlib.closing(B()) as xb:
        return xa.run() + xb.run()


def use_preserves_b(b: B):
    with nullcontext(b) as xb:
        return xb.run()
