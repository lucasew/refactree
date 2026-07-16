class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_reversed(items: list[A]):
    for a in reversed(items):
        a.execute()


def use_reversed_b(items: list[B]):
    for b in reversed(items):
        b.run()


def use_sorted(items: list[A]):
    for a in sorted(items, key=lambda x: 0):
        a.execute()


def use_list(items: list[A]):
    for a in list(items):
        a.execute()


def use_iter(items: list[A]):
    for a in iter(items):
        a.execute()


def use_nested(items: list[A]):
    for a in list(reversed(items)):
        a.execute()


def use_literal():
    for a in reversed([A()]):
        a.execute()
    for b in sorted([B()]):
        b.run()
