from collections import deque
import collections


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_extendleft():
    xs = deque()
    ys = deque()
    xs.extendleft([A()])
    ys.extendleft([B()])
    return xs[0].run() + ys[0].run()


def use_extendleft_for():
    xs = deque()
    ys = deque()
    xs.extendleft([A()])
    ys.extendleft([B()])
    n = 0
    for a in xs:
        n += a.run()
    for b in ys:
        n += b.run()
    return n


def use_extendleft_var():
    xs = deque()
    ys = deque()
    xs.extendleft([A()])
    ys.extendleft([B()])
    a = xs[0]
    b = ys[0]
    return a.run() + b.run()


def use_extendleft_tuple():
    xs = deque()
    ys = deque()
    xs.extendleft((A(),))
    ys.extendleft((B(),))
    return xs[0].run() + ys[0].run()


def use_collections_deque_extendleft():
    xs = collections.deque()
    ys = collections.deque()
    xs.extendleft([A()])
    ys.extendleft([B()])
    return xs[0].run() + ys[0].run()


def use_preserves_b():
    ys = deque()
    ys.extendleft([B()])
    return ys[0].run()
