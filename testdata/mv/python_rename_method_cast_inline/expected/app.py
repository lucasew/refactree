from typing import cast
import typing


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_inline(x, y):
    return cast(A, x).execute() + cast(B, y).run()


def use_typing(x, y):
    return typing.cast(A, x).execute() + typing.cast(B, y).run()


def use_assign(x, y):
    a = cast(A, x)
    b = cast(B, y)
    return a.execute() + b.run()


def use_preserves_b(y):
    return cast(B, y).run()
