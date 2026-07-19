from functools import partial
import functools


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_partial_call():
    return partial(A)().execute() + partial(B)().run()


def use_functools_partial():
    return functools.partial(A)().execute() + functools.partial(B)().run()


def use_preserves_b():
    return partial(B)().run()
