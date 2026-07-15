class Box:
    def get_value(self):
        return 1

    def twice(self):
        return self.get_value() + Box.get_value(self)


def use(b: Box):
    return b.get_value()


def make():
    box = Box()
    return box.get_value()
