class Box:
    VALUE = 1
    COUNT = 2

    def use(self):
        return self.VALUE + Box.VALUE
