from collections import deque


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_popleft(items: deque[A]):
    a = items.popleft()
    a.run()


def use_popleft_b(items: deque[B]):
    b = items.popleft()
    b.run()


def use_popleft_assigned():
    xs: deque[A] = deque([A()])
    a = xs.popleft()
    a.run()
    ys: deque[B] = deque([B()])
    b = ys.popleft()
    b.run()


def use_walrus_popleft(items: deque[A]):
    if (a := items.popleft()):
        a.run()
