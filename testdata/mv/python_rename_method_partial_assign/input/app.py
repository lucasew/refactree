from functools import partial
import functools


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_partial_assign():
    pa = partial(A)
    pb = partial(B)
    return pa().run() + pb().run()


def use_functools_partial_assign():
    pa = functools.partial(A)
    pb = functools.partial(B)
    return pa().run() + pb().run()


def use_preserves_b():
    pb = partial(B)
    return pb().run()
