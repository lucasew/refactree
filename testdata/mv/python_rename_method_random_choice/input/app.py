import random
from random import choice, choices, sample


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_choice(items: list[A]):
    a = random.choice(items)
    a.run()


def use_choice_imported(items: list[A]):
    a = choice(items)
    a.run()


def use_choices(items: list[A]):
    for a in random.choices(items, k=2):
        a.run()


def use_sample(items: list[A]):
    for a in sample(items, 1):
        a.run()


def use_choices_imported(items: list[A]):
    for a in choices(items, k=2):
        a.run()


def use_sample_mod(items: list[A]):
    for a in random.sample(items, 2):
        a.run()


def use_choice_b(items: list[B]):
    b = choice(items)
    b.run()


def use_choices_b(items: list[B]):
    for b in choices(items, k=1):
        b.run()


def use_choice_literal():
    a = choice([A()])
    a.run()
    b = random.choice([B()])
    b.run()


def use_choices_assigned():
    xs = [A()]
    for a in choices(xs, k=1):
        a.run()
    ys = [B()]
    for b in random.choices(ys, k=1):
        b.run()


def use_choices_bind(items: list[A]):
    top = random.choices(items, k=2)
    for a in top:
        a.run()


def use_sample_bind(items: list[A]):
    top = sample(items, 1)
    for a in top:
        a.run()


def use_choices_nested(items: list[A]):
    for a in list(choices(items, k=2)):
        a.run()


def use_choice_walrus(items: list[A]):
    if (a := choice(items)):
        a.run()
    if (a := random.choice(items)):
        a.run()


def use_choice_walrus_b(items: list[B]):
    if (b := random.choice(items)):
        b.run()
