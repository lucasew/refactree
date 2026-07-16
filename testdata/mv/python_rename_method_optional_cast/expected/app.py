from typing import Optional, Union, cast


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_optional(a: Optional[A]):
    if a is not None:
        a.execute()


def use_optional_b(b: Optional[B]):
    if b is not None:
        b.run()


def use_union_pipe(a: A | None):
    if a is not None:
        a.execute()


def use_union_pipe_b(b: B | None):
    if b is not None:
        b.run()


def use_union_typing(a: Union[A, None]):
    if a is not None:
        a.execute()


def use_union_typing_b(b: Union[B, None]):
    if b is not None:
        b.run()


def use_cast(x):
    a = cast(A, x)
    a.execute()


def use_cast_b(x):
    b = cast(B, x)
    b.run()


def use_typing_cast(x):
    a = typing_cast_alias(x)
    return a


def typing_cast_alias(x):
    # attribute form: typing.cast
    import typing

    a = typing.cast(A, x)
    a.execute()
    return a
