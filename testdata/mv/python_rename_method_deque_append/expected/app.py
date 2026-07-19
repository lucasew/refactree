from collections import deque
import collections


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_deque_append():
    xs = deque()
    ys = deque()
    xs.append(A())
    ys.append(B())
    return xs[0].execute() + ys[0].run()


def use_deque_append_for():
    xs = deque()
    ys = deque()
    xs.append(A())
    ys.append(B())
    n = 0
    for a in xs:
        n += a.execute()
    for b in ys:
        n += b.run()
    return n


def use_deque_appendleft():
    xs = deque()
    ys = deque()
    xs.appendleft(A())
    ys.appendleft(B())
    return xs[0].execute() + ys[0].run()


def use_collections_deque():
    xs = collections.deque()
    ys = collections.deque()
    xs.append(A())
    ys.append(B())
    return xs[0].execute() + ys[0].run()


def use_preserves_b():
    ys = deque()
    ys.append(B())
    return ys[0].run()
