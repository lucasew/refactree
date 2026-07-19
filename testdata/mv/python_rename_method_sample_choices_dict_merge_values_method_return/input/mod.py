import random
from random import choices, sample


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    a: A

    def __init__(self, a: A) -> None:
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    b: B

    def __init__(self, b: B) -> None:
        self.b = b

    def get(self) -> B:
        return self.b


# --- Class regressions: sample/choices + dict-merge values (already solid). ---
def use_class_sample_inline() -> int:
    return random.sample([A()], 1)[0].run() + random.sample([B()], 1)[0].run()


def use_class_sample_bare() -> int:
    return sample([A()], 1)[0].run() + sample([B()], 1)[0].run()


def use_class_choices_inline() -> int:
    return random.choices([A()], k=1)[0].run() + random.choices([B()], k=1)[0].run()


def use_class_choices_bare() -> int:
    return choices([A()], k=1)[0].run() + choices([B()], k=1)[0].run()


def use_class_sample_assign() -> int:
    xs = random.sample([A()], 1)
    ys = sample([B()], 1)
    return xs[0].run() + ys[0].run()


def use_class_choices_for() -> int:
    s = 0
    for a in random.choices([A()], k=1):
        s += a.run()
    for b in choices([B()], k=1):
        s += b.run()
    return s


def use_class_merge_values() -> int:
    return (
        list(({"k": A()} | {}).values())[0].run()
        + list(({} | {"k": B()}).values())[0].run()
    )


def use_class_merge_values_multi() -> int:
    return (
        list(({"k": A()} | {"j": A()}).values())[0].run()
        + list(({"k": B()} | {"j": B()}).values())[0].run()
    )


# --- Method-return under foreign same-leaf. ---
def use_mr_sample_inline(ba: BoxA, bb: BoxB) -> int:
    return random.sample([ba.get()], 1)[0].run() + random.sample([bb.get()], 1)[0].run()


def use_mr_sample_bare(ba: BoxA, bb: BoxB) -> int:
    return sample([ba.get()], 1)[0].run() + sample([bb.get()], 1)[0].run()


def use_mr_choices_inline(ba: BoxA, bb: BoxB) -> int:
    return random.choices([ba.get()], k=1)[0].run() + random.choices([bb.get()], k=1)[0].run()


def use_mr_choices_bare(ba: BoxA, bb: BoxB) -> int:
    return choices([ba.get()], k=1)[0].run() + choices([bb.get()], k=1)[0].run()


def use_mr_sample_assign(ba: BoxA, bb: BoxB) -> int:
    xs = random.sample([ba.get()], 1)
    ys = sample([bb.get()], 1)
    return xs[0].run() + ys[0].run()


def use_mr_choices_for(ba: BoxA, bb: BoxB) -> int:
    s = 0
    for a in random.choices([ba.get()], k=1):
        s += a.run()
    for b in choices([bb.get()], k=1):
        s += b.run()
    return s


def use_mr_merge_values(ba: BoxA, bb: BoxB) -> int:
    return (
        list(({"k": ba.get()} | {}).values())[0].run()
        + list(({} | {"k": bb.get()}).values())[0].run()
    )


def use_mr_merge_values_multi(ba: BoxA, bb: BoxB) -> int:
    return (
        list(({"k": ba.get()} | {"j": ba.get()}).values())[0].run()
        + list(({"k": bb.get()} | {"j": bb.get()}).values())[0].run()
    )


# choice already solid (regression).
def use_mr_choice(ba: BoxA, bb: BoxB) -> int:
    return random.choice([ba.get()]).run() + random.choice([bb.get()]).run()


def use_class_choice() -> int:
    return random.choice([A()]).run() + random.choice([B()]).run()


def use_preserves_b(bb: BoxB) -> int:
    return (
        random.sample([bb.get()], 1)[0].run()
        + random.choices([bb.get()], k=1)[0].run()
        + list(({"k": bb.get()} | {}).values())[0].run()
        + sample([bb.get()], 1)[0].run()
        + choices([bb.get()], k=1)[0].run()
    )
