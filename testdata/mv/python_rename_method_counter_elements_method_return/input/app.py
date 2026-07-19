from collections import Counter


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


class BoxA:
    def __init__(self, a: A):
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b


def use_elements(ba: BoxA, bb: BoxB):
    return list(Counter([ba.get()]).elements())[0].run() + list(
        Counter([bb.get()]).elements()
    )[0].run()


def use_collections_counter(ba: BoxA, bb: BoxB):
    return list(Counter([ba.get()]))[0].run() + list(Counter([bb.get()]))[0].run()


def use_assign(ba: BoxA, bb: BoxB):
    xa = list(Counter([ba.get()]).elements())[0]
    xb = list(Counter([bb.get()]).elements())[0]
    return xa.run() + xb.run()


def use_class():
    return list(Counter([A()]).elements())[0].run() + list(
        Counter([B()]).elements()
    )[0].run()


def use_preserves_b(bb: BoxB):
    return list(Counter([bb.get()]).elements())[0].run() + list(
        Counter([B()]).elements()
    )[0].run()
